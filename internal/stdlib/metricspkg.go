package stdlib

import (
	"math"
	"sort"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageMetrics: ML evaluation metrics — accuracy, F1, precision, recall,
// confusion matrix, BLEU, ROUGE-L, cosine similarity, MSE, MAE, R².
// All pure Go, no dependencies.
func packageMetrics() runtime.Value {
	p := pkg()

	// metrics.accuracy(true_labels, predicted) -> float
	set(p, "accuracy", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.accuracy(true, predicted)", "metrics"), nil
		}
		y, p := toStrSlice(args[0]), toStrSlice(args[1])
		if len(y) == 0 || len(y) != len(p) {
			return errRes("arrays must be same non-zero length", "metrics"), nil
		}
		correct := 0
		for i := range y {
			if y[i] == p[i] {
				correct++
			}
		}
		return runtime.Float(float64(correct) / float64(len(y))), nil
	}, 2)

	// metrics.precision(true, predicted, label?) -> float
	set(p, "precision", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.precision(true, predicted, label?)", "metrics"), nil
		}
		y, p := toStrSlice(args[0]), toStrSlice(args[1])
		label := "1"
		if len(args) >= 3 {
			label = args[2].String()
		}
		tp, fp := 0, 0
		for i := range y {
			if p[i] == label {
				if y[i] == label {
					tp++
				} else {
					fp++
				}
			}
		}
		if tp+fp == 0 {
			return runtime.Float(0), nil
		}
		return runtime.Float(float64(tp) / float64(tp+fp)), nil
	}, 3)

	// metrics.recall(true, predicted, label?) -> float
	set(p, "recall", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.recall(true, predicted, label?)", "metrics"), nil
		}
		y, p := toStrSlice(args[0]), toStrSlice(args[1])
		label := "1"
		if len(args) >= 3 {
			label = args[2].String()
		}
		tp, fn := 0, 0
		for i := range y {
			if y[i] == label {
				if p[i] == label {
					tp++
				} else {
					fn++
				}
			}
		}
		if tp+fn == 0 {
			return runtime.Float(0), nil
		}
		return runtime.Float(float64(tp) / float64(tp+fn)), nil
	}, 3)

	// metrics.f1(true, predicted, label?) -> float
	set(p, "f1", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.f1(true, predicted, label?)", "metrics"), nil
		}
		y, pred := toStrSlice(args[0]), toStrSlice(args[1])
		label := "1"
		if len(args) >= 3 {
			label = args[2].String()
		}
		tp, fp, fn := 0, 0, 0
		for i := range y {
			if pred[i] == label {
				if y[i] == label {
					tp++
				} else {
					fp++
				}
			} else if y[i] == label {
				fn++
			}
		}
		prec := float64(0)
		if tp+fp > 0 {
			prec = float64(tp) / float64(tp+fp)
		}
		rec := float64(0)
		if tp+fn > 0 {
			rec = float64(tp) / float64(tp+fn)
		}
		if prec+rec == 0 {
			return runtime.Float(0), nil
		}
		return runtime.Float(2 * prec * rec / (prec + rec)), nil
	}, 3)

	// metrics.confusion_matrix(true, predicted) -> map {labels, matrix}
	set(p, "confusion_matrix", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.confusion_matrix(true, predicted)", "metrics"), nil
		}
		y, pred := toStrSlice(args[0]), toStrSlice(args[1])
		labels := uniqueLabels(y, pred)
		sort.Strings(labels)
		idx := map[string]int{}
		for i, l := range labels {
			idx[l] = i
		}
		n := len(labels)
		matrix := make([][]int, n)
		for i := range matrix {
			matrix[i] = make([]int, n)
		}
		for i := range y {
			ri, ci := idx[y[i]], idx[pred[i]]
			matrix[ri][ci]++
		}
		// Convert to Weft values
		labelVals := make([]runtime.Value, len(labels))
		for i, l := range labels {
			labelVals[i] = runtime.Str(l)
		}
		rowVals := make([]runtime.Value, n)
		for i, row := range matrix {
			cells := make([]runtime.Value, n)
			for j, v := range row {
				cells[j] = runtime.Int(int64(v))
			}
			rowVals[i] = runtime.List(cells...)
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"labels", "matrix"}
		mo.Vals["labels"] = runtime.List(labelVals...)
		mo.Vals["matrix"] = runtime.List(rowVals...)
		return m, nil
	}, 2)

	// metrics.classification_report(true, predicted) -> map per label
	set(p, "classification_report", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.classification_report(true, predicted)", "metrics"), nil
		}
		y, pred := toStrSlice(args[0]), toStrSlice(args[1])
		labels := uniqueLabels(y, pred)
		sort.Strings(labels)
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		for _, label := range labels {
			tp, fp, fn := 0, 0, 0
			for i := range y {
				if pred[i] == label {
					if y[i] == label {
						tp++
					} else {
						fp++
					}
				} else if y[i] == label {
					fn++
				}
			}
			prec := float64(0)
			if tp+fp > 0 {
				prec = float64(tp) / float64(tp+fp)
			}
			rec := float64(0)
			if tp+fn > 0 {
				rec = float64(tp) / float64(tp+fn)
			}
			f1 := float64(0)
			if prec+rec > 0 {
				f1 = 2 * prec * rec / (prec + rec)
			}
			entry := runtime.NewMap()
			eo := entry.Obj.(*runtime.MapObj)
			eo.Keys = []string{"precision", "recall", "f1", "support"}
			eo.Vals["precision"] = runtime.Float(prec)
			eo.Vals["recall"] = runtime.Float(rec)
			eo.Vals["f1"] = runtime.Float(f1)
			eo.Vals["support"] = runtime.Int(int64(tp + fn))
			mo.Keys = append(mo.Keys, label)
			mo.Vals[label] = entry
		}
		return m, nil
	}, 2)

	// metrics.bleu(reference, hypothesis, n?) -> float  (sentence-level BLEU)
	set(p, "bleu", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.bleu(reference, hypothesis, n?)", "metrics"), nil
		}
		ref := strings.Fields(args[0].String())
		hyp := strings.Fields(args[1].String())
		maxN := 4
		if len(args) >= 3 {
			if n, err := runtime.AsInt(args[2]); err == nil && n > 0 {
				maxN = int(n)
			}
		}
		return runtime.Float(sentenceBLEU(ref, hyp, maxN)), nil
	}, 3)

	// metrics.rouge_l(reference, hypothesis) -> float (ROUGE-L F1)
	set(p, "rouge_l", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.rouge_l(reference, hypothesis)", "metrics"), nil
		}
		ref := strings.Fields(args[0].String())
		hyp := strings.Fields(args[1].String())
		return runtime.Float(rougeL(ref, hyp)), nil
	}, 2)

	// metrics.cosine(a, b) -> float  (cosine similarity of two float lists)
	set(p, "cosine", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.cosine(a, b)", "metrics"), nil
		}
		a := toFloatSlice(args[0])
		b := toFloatSlice(args[1])
		return runtime.Float(cosineSim(a, b)), nil
	}, 2)

	// metrics.mse(true, predicted) -> float
	set(p, "mse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.mse(true, predicted)", "metrics"), nil
		}
		y := toFloatSlice(args[0])
		p := toFloatSlice(args[1])
		if len(y) == 0 {
			return runtime.Float(0), nil
		}
		var sum float64
		for i := range y {
			d := y[i] - p[i]
			sum += d * d
		}
		return runtime.Float(sum / float64(len(y))), nil
	}, 2)

	// metrics.mae(true, predicted) -> float
	set(p, "mae", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.mae(true, predicted)", "metrics"), nil
		}
		y := toFloatSlice(args[0])
		p := toFloatSlice(args[1])
		if len(y) == 0 {
			return runtime.Float(0), nil
		}
		var sum float64
		for i := range y {
			sum += math.Abs(y[i] - p[i])
		}
		return runtime.Float(sum / float64(len(y))), nil
	}, 2)

	// metrics.r2(true, predicted) -> float (R² coefficient of determination)
	set(p, "r2", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("metrics.r2(true, predicted)", "metrics"), nil
		}
		y := toFloatSlice(args[0])
		p := toFloatSlice(args[1])
		if len(y) == 0 {
			return runtime.Float(0), nil
		}
		var mean float64
		for _, v := range y {
			mean += v
		}
		mean /= float64(len(y))
		var ssRes, ssTot float64
		for i := range y {
			ssRes += (y[i] - p[i]) * (y[i] - p[i])
			ssTot += (y[i] - mean) * (y[i] - mean)
		}
		if ssTot == 0 {
			return runtime.Float(0), nil
		}
		return runtime.Float(1 - ssRes/ssTot), nil
	}, 2)

	return p
}

func toStrSlice(v runtime.Value) []string {
	if v.Kind != runtime.KindList {
		return nil
	}
	items := v.Obj.(*runtime.ListObj).Items
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.String()
	}
	return out
}

func toFloatSlice(v runtime.Value) []float64 {
	if v.Kind != runtime.KindList {
		return nil
	}
	items := v.Obj.(*runtime.ListObj).Items
	out := make([]float64, len(items))
	for i, it := range items {
		switch it.Kind {
		case runtime.KindFloat:
			out[i] = it.F
		case runtime.KindInt:
			out[i] = float64(it.I)
		}
	}
	return out
}

func uniqueLabels(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sentenceBLEU(ref, hyp []string, maxN int) float64 {
	if len(hyp) == 0 {
		return 0
	}
	var logSum float64
	for n := 1; n <= maxN; n++ {
		refNgrams := ngrams(ref, n)
		hypNgrams := ngrams(hyp, n)
		match := 0
		for ng, cnt := range hypNgrams {
			if rc, ok := refNgrams[ng]; ok {
				if cnt < rc {
					match += cnt
				} else {
					match += rc
				}
			}
		}
		total := len(hyp) - n + 1
		if total <= 0 || match == 0 {
			return 0
		}
		logSum += math.Log(float64(match) / float64(total))
	}
	// brevity penalty
	bp := 1.0
	if len(hyp) < len(ref) {
		bp = math.Exp(1 - float64(len(ref))/float64(len(hyp)))
	}
	return bp * math.Exp(logSum/float64(maxN))
}

func ngrams(words []string, n int) map[string]int {
	m := map[string]int{}
	for i := 0; i <= len(words)-n; i++ {
		key := strings.Join(words[i:i+n], " ")
		m[key]++
	}
	return m
}

func rougeL(ref, hyp []string) float64 {
	if len(ref) == 0 || len(hyp) == 0 {
		return 0
	}
	lcsLen := lcs(ref, hyp)
	prec := float64(lcsLen) / float64(len(hyp))
	rec := float64(lcsLen) / float64(len(ref))
	if prec+rec == 0 {
		return 0
	}
	return 2 * prec * rec / (prec + rec)
}

func lcs(a, b []string) int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	return dp[m][n]
}

func cosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	denom := math.Sqrt(na) * math.Sqrt(nb)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
