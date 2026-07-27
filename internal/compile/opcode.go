package compile

// Op is a bytecode opcode.
type Op byte

const (
	OpLoadConst Op = iota
	OpLoadLocal
	OpStoreLocal
	OpLoadGlobal
	OpStoreGlobal
	OpPop
	OpTrue
	OpFalse
	OpNull
	OpUnit
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpNeg
	OpNot
	OpEq
	OpNeq
	OpLt
	OpLte
	OpGt
	OpGte
	OpAnd // unused — short-circuit via jumps
	OpOr  // unused
	OpNullCoalesce
	OpJump
	OpJumpIfFalse    // peek; jump if falsey (&& short-circuit)
	OpJumpIfTrue     // peek; jump if truthy (|| short-circuit)
	OpJumpIfFalsePop // pop; jump if was falsey (if/while/for)
	OpJumpIfTruePop  // pop; jump if was truthy
	OpCall
	OpReturn
	OpDefer // pop argc args + callee; schedule LIFO call on frame exit
	OpMakeList
	OpMakeMap
	OpMakeStruct
	OpGetIndex
	OpSetIndex
	OpGetField
	OpSetField
	OpTryQ
	OpWrapResult // bare T → Ok(T); Error → Err; Result unchanged
	OpIterNew
	OpIterNext
	OpConcat
	OpClose // pop n upvalues then func; push Func with Upvalues (by-value capture)
)

func (op Op) String() string {
	names := []string{
		"LOAD_CONST", "LOAD_LOCAL", "STORE_LOCAL", "LOAD_GLOBAL", "STORE_GLOBAL",
		"POP", "TRUE", "FALSE", "NULL", "UNIT",
		"ADD", "SUB", "MUL", "DIV", "MOD", "NEG", "NOT",
		"EQ", "NEQ", "LT", "LTE", "GT", "GTE", "AND", "OR", "NULL_COALESCE",
		"JUMP", "JUMP_IF_FALSE", "JUMP_IF_TRUE", "JUMP_IF_FALSE_POP", "JUMP_IF_TRUE_POP",
		"CALL", "RETURN", "DEFER",
		"MAKE_LIST", "MAKE_MAP", "MAKE_STRUCT", "GET_INDEX", "SET_INDEX",
		"GET_FIELD", "SET_FIELD", "TRY_Q", "WRAP_RESULT", "ITER_NEW", "ITER_NEXT", "CONCAT",
		"CLOSE",
	}
	if int(op) < len(names) {
		return names[op]
	}
	return "?"
}

// Chunk is a compiled function body.
type Chunk struct {
	Code    []byte
	Consts  []any
	Lines   []int
	NumLocs int
	Name    string
	Arity   int
	File    string // source path for stack traces
}
