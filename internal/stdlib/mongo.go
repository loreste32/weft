//go:build !js && !slim

package stdlib

import (
	"context"
	"time"

	"github.com/loreste/weft/internal/runtime"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// packageMongo — MongoDB NoSQL.
//
//	m := mongo.connect("mongodb://localhost:27017")?
//	col := m.collection("app", "users")
//	col.insert({"name": "Ada"})?
//	docs := col.find({"active": true})?
func packageMongo(env *runtime.Env) runtime.Value {
	p := pkg()

	set(p, "connect", func(args []runtime.Value) (runtime.Value, error) {
		uri := "mongodb://127.0.0.1:27017"
		if len(args) >= 1 && args[0].String() != "" {
			uri = args[0].String()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		if err := client.Ping(ctx, nil); err != nil {
			_ = client.Disconnect(context.Background())
			return errRes(err.Error(), "mongo"), nil
		}
		return runtime.Ok(wrapMongo(client)), nil
	}, 1)

	return p
}

func wrapMongo(client *mongo.Client) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("mongo."+name, arity, fn)
	}

	// m.collection(db, name) -> collection handle
	put("collection", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("m.collection(db, name)", "mongo"), nil
		}
		col := client.Database(args[0].String()).Collection(args[1].String())
		return wrapMongoColl(col), nil
	})

	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Disconnect(ctx); err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	put("ping", 0, func(args []runtime.Value) (runtime.Value, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := client.Ping(ctx, nil); err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		return runtime.Ok(runtime.Bool(true)), nil
	})

	return m
}

func wrapMongoColl(col *mongo.Collection) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("mongo.col."+name, arity, fn)
	}

	put("insert", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("col.insert(doc)", "mongo"), nil
		}
		doc := valueToBSON(args[0])
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := col.InsertOne(ctx, doc)
		if err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		out := runtime.NewMap()
		omo := out.Obj.(*runtime.MapObj)
		omo.Keys = []string{"id"}
		omo.Vals["id"] = runtime.Str(stringifyID(res.InsertedID))
		return runtime.Ok(out), nil
	})

	put("insert_many", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("col.insert_many([doc…])", "mongo"), nil
		}
		var docs []any
		for _, it := range args[0].Obj.(*runtime.ListObj).Items {
			docs = append(docs, valueToBSON(it))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := col.InsertMany(ctx, docs)
		if err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		return runtime.Ok(runtime.Int(int64(len(res.InsertedIDs)))), nil
	})

	// col.find(filter?, opts?) -> Result[[map]]
	put("find", 2, func(args []runtime.Value) (runtime.Value, error) {
		filter := bson.M{}
		if len(args) >= 1 && args[0].Kind != runtime.KindNull {
			filter = valueToBSONMap(args[0])
		}
		findOpts := options.Find()
		if len(args) >= 2 {
			if n := mapGetInt(args[1], "limit", 0); n > 0 {
				findOpts.SetLimit(n)
			}
			if n := mapGetInt(args[1], "skip", 0); n > 0 {
				findOpts.SetSkip(n)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cur, err := col.Find(ctx, filter, findOpts)
		if err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		defer cur.Close(ctx)
		var results []runtime.Value
		for cur.Next(ctx) {
			var raw bson.M
			if err := cur.Decode(&raw); err != nil {
				return errRes(err.Error(), "mongo"), nil
			}
			results = append(results, bsonToValue(raw))
		}
		if err := cur.Err(); err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		return runtime.Ok(runtime.List(results...)), nil
	})

	put("find_one", 1, func(args []runtime.Value) (runtime.Value, error) {
		filter := bson.M{}
		if len(args) >= 1 && args[0].Kind != runtime.KindNull {
			filter = valueToBSONMap(args[0])
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var raw bson.M
		err := col.FindOne(ctx, filter).Decode(&raw)
		if err == mongo.ErrNoDocuments {
			return runtime.Ok(runtime.Null()), nil
		}
		if err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		return runtime.Ok(bsonToValue(raw)), nil
	})

	put("update", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("col.update(filter, update, many?)", "mongo"), nil
		}
		filter := valueToBSONMap(args[0])
		update := valueToBSONMap(args[1])
		// wrap in $set if no operator keys
		if !hasMongoOp(update) {
			update = bson.M{"$set": update}
		}
		many := false
		if len(args) >= 3 && args[2].Kind == runtime.KindBool {
			many = args[2].B
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var n int64
		if many {
			res, err := col.UpdateMany(ctx, filter, update)
			if err != nil {
				return errRes(err.Error(), "mongo"), nil
			}
			n = res.ModifiedCount
		} else {
			res, err := col.UpdateOne(ctx, filter, update)
			if err != nil {
				return errRes(err.Error(), "mongo"), nil
			}
			n = res.ModifiedCount
		}
		return runtime.Ok(runtime.Int(n)), nil
	})

	put("delete", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("col.delete(filter, many?)", "mongo"), nil
		}
		filter := valueToBSONMap(args[0])
		many := false
		if len(args) >= 2 && args[1].Kind == runtime.KindBool {
			many = args[1].B
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var n int64
		if many {
			res, err := col.DeleteMany(ctx, filter)
			if err != nil {
				return errRes(err.Error(), "mongo"), nil
			}
			n = res.DeletedCount
		} else {
			res, err := col.DeleteOne(ctx, filter)
			if err != nil {
				return errRes(err.Error(), "mongo"), nil
			}
			n = res.DeletedCount
		}
		return runtime.Ok(runtime.Int(n)), nil
	})

	put("count", 1, func(args []runtime.Value) (runtime.Value, error) {
		filter := bson.M{}
		if len(args) >= 1 && args[0].Kind != runtime.KindNull {
			filter = valueToBSONMap(args[0])
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		n, err := col.CountDocuments(ctx, filter)
		if err != nil {
			return errRes(err.Error(), "mongo"), nil
		}
		return runtime.Ok(runtime.Int(n)), nil
	})

	return m
}

func valueToBSON(v runtime.Value) any {
	return valueToGo(v)
}

func valueToBSONMap(v runtime.Value) bson.M {
	g := valueToGo(v)
	if m, ok := g.(map[string]any); ok {
		return bson.M(m)
	}
	return bson.M{}
}

func hasMongoOp(m bson.M) bool {
	for k := range m {
		if len(k) > 0 && k[0] == '$' {
			return true
		}
	}
	return false
}

func bsonToValue(m bson.M) runtime.Value {
	// convert via map[string]any
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = bsonSanitize(v)
	}
	return goToValue(out)
}

func bsonSanitize(v any) any {
	switch x := v.(type) {
	case bson.M:
		m := make(map[string]any, len(x))
		for k, vv := range x {
			m[k] = bsonSanitize(vv)
		}
		return m
	case bson.A:
		a := make([]any, len(x))
		for i, vv := range x {
			a[i] = bsonSanitize(vv)
		}
		return a
	case bson.D:
		m := make(map[string]any, len(x))
		for _, e := range x {
			m[e.Key] = bsonSanitize(e.Value)
		}
		return m
	default:
		return x
	}
}

func stringifyID(id any) string {
	if id == nil {
		return ""
	}
	// ObjectID has Hex()
	type hexer interface{ Hex() string }
	if h, ok := id.(hexer); ok {
		return h.Hex()
	}
	return fmtSprint(id)
}
