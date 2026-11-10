// ============================================================================
// [LOCKED CORE ENGINE] harness.go — Perspective-Aware Verification Harness v3.0 (Go Edition)
// ============================================================================
// PERINGATAN UNTUK AGEN AI:
// File ini adalah mesin verifikasi inti yang bersifat IMMUTABLE (TIDAK BOLEH DIUBAH).
// Jangan memodifikasi komparator, aturan gerbang, atau logika `IsCompleteAndVerified`
// di dalam file ini. Seluruh penyesuaian untuk fungsi/modul baru WAJIB dilakukan
// hanya pada file spesifikasi (`suiteSpec.template.go` / `*_spec.go`).

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime/debug"
	"strings"
	"time"
)

// ============================================================================
// BAGIAN 1: TIPE DATA SPESIFIKASI & LAPORAN VERIFIKASI (GENERICS Go 1.18+)
// ============================================================================

// SubjectFunc merepresentasikan tanda tangan fungsi target di Go:
// menerima 1 payload bertipe TIn dan mengembalikan (TOut, error).
type SubjectFunc[TIn any, TOut any] func(input TIn) (TOut, error)

// ComparatorFunc membandingkan nilai aktual dan ekspektasi.
type ComparatorFunc[TOut any] func(actual, expected TOut) bool

// SuiteOptions mengatur parameter keketatan mesin verifikasi.
type SuiteOptions struct {
	EnforceImmutability    bool    `json:"enforceImmutability"`
	RequireAllPerspectives bool    `json:"requireAllPerspectives"`
	DeterminismRuns        int     `json:"determinismRuns"`
	MaxDurationMs          float64 `json:"maxDurationMs,omitempty"`
	StopOnFirstOracleDrift bool    `json:"stopOnFirstOracleDrift"`
	MaxOracleFailures      int     `json:"maxOracleFailures"`
}

// DefaultSuiteOptions mengembalikan opsi bawaan yang ketat (anti-vacuous & anti-mutasi).
func DefaultSuiteOptions() SuiteOptions {
	return SuiteOptions{
		EnforceImmutability:    true,
		RequireAllPerspectives: true,
		DeterminismRuns:        2,
		MaxDurationMs:          50.0,
		StopOnFirstOracleDrift: false,
		MaxOracleFailures:      10,
	}
}

// PrimaryCase mendefinisikan kasus uji pada Gerbang 1 (Primary-Target).
type PrimaryCase[TIn any, TOut any] struct {
	Name                string  `json:"name"`
	Input               TIn     `json:"input"`
	Expected            TOut    `json:"expected"`
	EnforceImmutability *bool   `json:"enforceImmutability,omitempty"`
	MaxDurationMs       float64 `json:"maxDurationMs,omitempty"`
}

// PoisonCase mendefinisikan kasus uji ekstrem/racun pada Gerbang 2 (Boundary-And-Poison).
type PoisonCase[TIn any, TOut any] struct {
	Name                  string                            `json:"name"`
	Input                 TIn                               `json:"input"`
	ExpectError           bool                              `json:"expectError"`
	ExpectedErrorContains string                            `json:"expectedErrorContains,omitempty"`
	ErrorMatcher          func(err error) bool              `json:"-"`
	AllowPanic            bool                              `json:"allowPanic,omitempty"`
	InvariantCheck        func(actual TOut, input TIn) bool `json:"-"`
	FailureHint           string                            `json:"failureHint,omitempty"`
	EnforceImmutability   *bool                             `json:"enforceImmutability,omitempty"`
	MaxDurationMs         float64                           `json:"maxDurationMs,omitempty"`
}

// RegressionCase mendefinisikan kasus uji pada Gerbang 4 (Post-PASS-Regression).
type RegressionCase[TIn any, TOut any] struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Input               TIn     `json:"input"`
	Expected            TOut    `json:"expected"`
	EnforceImmutability *bool   `json:"enforceImmutability,omitempty"`
	MaxDurationMs       float64 `json:"maxDurationMs,omitempty"`
}

// SuiteSpec adalah kontrak utama spesifikasi pengujian 4 gerbang.
type SuiteSpec[TIn any, TOut any] struct {
	Name             string                      `json:"name"`
	Domain           string                      `json:"domain"` // "low-level" | "general-app" | "boundary"
	Subject          SubjectFunc[TIn, TOut]      `json:"-"`
	ReferenceOracle  SubjectFunc[TIn, TOut]      `json:"-"`
	Comparator       ComparatorFunc[TOut]        `json:"-"`
	Options          *SuiteOptions               `json:"options,omitempty"`
	PrimaryCases     []PrimaryCase[TIn, TOut]    `json:"primaryCases"`
	PoisonInputs     []PoisonCase[TIn, TOut]     `json:"poisonInputs"`
	GeneratedSamples []TIn                       `json:"generatedSamples"`
	RegressionSuite  []RegressionCase[TIn, TOut] `json:"regressionSuite"`
}

// StructuralDiff merekam perbedaan spesifik per-path antara nilai aktual dan ekspektasi.
type StructuralDiff struct {
	Path             string   `json:"path"`
	MismatchKind     string   `json:"mismatchKind"`
	ExpectedType     string   `json:"expectedType"`
	ActualType       string   `json:"actualType"`
	ExpectedValue    any      `json:"expectedValue"`
	ActualValue      any      `json:"actualValue"`
	NumericDelta     *float64 `json:"numericDelta,omitempty"`
	AmbiguityWarning string   `json:"ambiguityWarning,omitempty"`
}

// SerializedError menyusun informasi error atau panic secara terstruktur.
type SerializedError struct {
	Kind    string `json:"kind"` // "Error" | "Panic"
	Type    string `json:"type"`
	Message string `json:"message"`
	Stack   string `json:"stack,omitempty"`
}

// FailureEvidence merekam bukti lengkap setiap kegagalan pada gerbang mana pun.
type FailureEvidence struct {
	Perspective     string           `json:"perspective"`
	Classification  string           `json:"classification"`
	RegressionID    string           `json:"regressionId,omitempty"`
	CaseName        string           `json:"caseName,omitempty"`
	SampleIndex     *int             `json:"sampleIndex,omitempty"`
	Input           any              `json:"input,omitempty"`
	MutatedInput    any              `json:"mutatedInput,omitempty"`
	Expected        any              `json:"expected,omitempty"`
	Actual          any              `json:"actual,omitempty"`
	FirstRunActual  any              `json:"firstRunActual,omitempty"`
	RepeatRunActual any              `json:"repeatRunActual,omitempty"`
	StructuralDiff  []StructuralDiff `json:"structuralDiff,omitempty"`
	MutationDiff    []StructuralDiff `json:"mutationDiff,omitempty"`
	Error           *SerializedError `json:"error,omitempty"`
	DurationMs      float64          `json:"durationMs,omitempty"`
	BudgetMs        float64          `json:"budgetMs,omitempty"`
	Detail          string           `json:"detail,omitempty"`
}

// RegressionCandidate adalah kandidat regresi otomatis yang diekstrak dari kegagalan baru.
type RegressionCandidate struct {
	ID                string           `json:"id"`
	SourcePerspective string           `json:"sourcePerspective"`
	Classification    string           `json:"classification"`
	Name              string           `json:"name"`
	Input             any              `json:"input"`
	Expected          any              `json:"expected,omitempty"`
	CapturedError     *SerializedError `json:"capturedError,omitempty"`
}

// BlindSpotAlert memperingatkan agen terhadap fenomena yang sering memicu kebutaan analisis.
type BlindSpotAlert struct {
	Code                 string   `json:"code"`
	Severity             string   `json:"severity"`
	CaseName             string   `json:"caseName,omitempty"`
	Path                 string   `json:"path,omitempty"`
	MutatedPaths         []string `json:"mutatedPaths,omitempty"`
	ViolatedPerspectives []string `json:"violatedPerspectives,omitempty"`
	RegressionIDs        []string `json:"regressionIds,omitempty"`
	Explanation          string   `json:"explanation"`
}

// SampleDiffEntry merangkum diff struktural untuk direktif perbaikan.
type SampleDiffEntry struct {
	Case  string           `json:"case"`
	Diffs []StructuralDiff `json:"diffs"`
}

// RepairDirective memberikan instruksi perbaikan berprioritas bagi agen AI.
type RepairDirective struct {
	Priority      int               `json:"priority"`
	ActionCode    string            `json:"actionCode"`
	TargetPaths   []string          `json:"targetPaths,omitempty"`
	RegressionIDs []string          `json:"regressionIds,omitempty"`
	SampleDiffs   []SampleDiffEntry `json:"sampleDiffs,omitempty"`
	Instruction   string            `json:"instruction"`
}

// AgentDiagnosticContext adalah konteks anti-kebutaan dan anti-ambiguitas untuk agen AI.
type AgentDiagnosticContext struct {
	NextActionDirective   string            `json:"nextActionDirective"`
	BlastRadius           string            `json:"blastRadius"`
	FalseGreenDetected    bool              `json:"falseGreenDetected"`
	ClassificationSummary map[string]int    `json:"classificationSummary"`
	BlindSpotAlerts       []BlindSpotAlert  `json:"blindSpotAlerts"`
	RepairDirectives      []RepairDirective `json:"repairDirectives"`
}

// GateCounts mencatat jumlah kasus uji yang terdaftar di masing-masing gerbang.
type GateCounts struct {
	Primary      int `json:"primary"`
	Poison       int `json:"poison"`
	Differential int `json:"differential"`
	Regression   int `json:"regression"`
}

// SuiteMetrics mencatat telemetri eksekusi pengujian.
type SuiteMetrics struct {
	TotalCasesExecuted int        `json:"totalCasesExecuted"`
	TotalDurationMs    float64    `json:"totalDurationMs"`
	MaxCaseDurationMs  float64    `json:"maxCaseDurationMs"`
	GateCounts         GateCounts `json:"gateCounts"`
}

// VerificationReport adalah hasil akhir eksekusi RunAgentVerificationSuite.
type VerificationReport struct {
	Module                        string                 `json:"module"`
	Domain                        string                 `json:"domain"`
	PrimaryPassed                 bool                   `json:"primaryPassed"`
	PerspectivesChecked           []string               `json:"perspectivesChecked"`
	UnverifiedPerspectives        []string               `json:"unverifiedPerspectives"`
	Metrics                       SuiteMetrics           `json:"metrics"`
	Failures                      []FailureEvidence      `json:"failures"`
	GeneratedRegressionCandidates []RegressionCandidate  `json:"generatedRegressionCandidates"`
	AgentDiagnosticContext        AgentDiagnosticContext `json:"agentDiagnosticContext"`
	IsCompleteAndVerified         bool                   `json:"isCompleteAndVerified"`
}

// ============================================================================
// BAGIAN 2: DEEP CLONE, KOMPARATOR, & DETEKSI AMBIGUITAS STRUKTURAL (REFLECT)
// ============================================================================

// SafeClone melakukan penyalinan mendalam (deep copy) menggunakan refleksi Go
// agar slice, map, dan pointer di dalam struct tidak berbagi referensi memori.
func SafeClone[T any](val T) T {
	cloned := deepCloneReflect(reflect.ValueOf(val))
	if !cloned.IsValid() {
		var zero T
		return zero
	}
	return cloned.Interface().(T)
}

func deepCloneReflect(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		ptr := reflect.New(v.Type().Elem())
		ptr.Elem().Set(deepCloneReflect(v.Elem()))
		return ptr

	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		inner := deepCloneReflect(v.Elem())
		iface := reflect.New(v.Type()).Elem()
		iface.Set(inner)
		return iface

	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		slice := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			slice.Index(i).Set(deepCloneReflect(v.Index(i)))
		}
		return slice

	case reflect.Array:
		arr := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			arr.Index(i).Set(deepCloneReflect(v.Index(i)))
		}
		return arr

	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		m := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			k := deepCloneReflect(iter.Key())
			val := deepCloneReflect(iter.Value())
			m.SetMapIndex(k, val)
		}
		return m

	case reflect.Struct:
		st := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			if st.Field(i).CanSet() {
				st.Field(i).Set(deepCloneReflect(v.Field(i)))
			} else {
				// Salin struct apa adanya jika memiliki unexported field
				st.Set(v)
				break
			}
		}
		return st

	default:
		return v
	}
}

// DefaultComparator membandingkan dua nilai Go secara mendalam dan menangani NaN secara aman.
func DefaultComparator[T any](actual, expected T) bool {
	return len(ComputeStructuralDiff(actual, expected, "$")) == 0
}

func describeGoType(v reflect.Value) string {
	if !v.IsValid() {
		return "nil"
	}
	if v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsNaN(f) {
			return fmt.Sprintf("%s(NaN)", v.Type().String())
		}
		if math.IsInf(f, 1) {
			return fmt.Sprintf("%s(+Inf)", v.Type().String())
		}
		if math.IsInf(f, -1) {
			return fmt.Sprintf("%s(-Inf)", v.Type().String())
		}
		if f == 0 && math.Signbit(f) {
			return fmt.Sprintf("%s(-0.0)", v.Type().String())
		}
		return v.Type().String()
	case reflect.Slice:
		if v.IsNil() {
			return fmt.Sprintf("%s(nil)", v.Type().String())
		}
		return fmt.Sprintf("%s(len=%d)", v.Type().String(), v.Len())
	case reflect.Map:
		if v.IsNil() {
			return fmt.Sprintf("%s(nil)", v.Type().String())
		}
		return fmt.Sprintf("%s(len=%d)", v.Type().String(), v.Len())
	case reflect.Ptr:
		if v.IsNil() {
			return fmt.Sprintf("%s(nil)", v.Type().String())
		}
		return v.Type().String()
	default:
		return v.Type().String()
	}
}

func safeInterface(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	if v.CanInterface() {
		if v.Kind() == reflect.Float32 || v.Kind() == reflect.Float64 {
			f := v.Float()
			if math.IsNaN(f) {
				return "NaN"
			}
			if math.IsInf(f, 1) {
				return "+Inf"
			}
			if math.IsInf(f, -1) {
				return "-Inf"
			}
		}
		return v.Interface()
	}
	return fmt.Sprintf("<unexported:%s>", v.Type().String())
}

// ComputeStructuralDiff melacak perbedaan nilai dan struktur di Go, termasuk jebakan khas Go:
// - Nil Slice vs Empty Slice (`[]int(nil)` vs `[]int{}`)
// - Kontaminasi NaN / Inf pada float64
// - Drift pembulatan desimal (< 0.01)
// - Jebakan representasi string identik (`"100"` vs `100`)
func ComputeStructuralDiff(actual, expected any, currentPath string) []StructuralDiff {
	var diffs []StructuralDiff
	computeDiffReflect(reflect.ValueOf(actual), reflect.ValueOf(expected), currentPath, &diffs)
	return diffs
}

func computeDiffReflect(actVal, expVal reflect.Value, path string, diffs *[]StructuralDiff) {
	for actVal.IsValid() && actVal.Kind() == reflect.Interface {
		if actVal.IsNil() {
			actVal = reflect.Value{}
			break
		}
		actVal = actVal.Elem()
	}
	for expVal.IsValid() && expVal.Kind() == reflect.Interface {
		if expVal.IsNil() {
			expVal = reflect.Value{}
			break
		}
		expVal = expVal.Elem()
	}

	if !actVal.IsValid() && !expVal.IsValid() {
		return
	}
	if !actVal.IsValid() || !expVal.IsValid() {
		*diffs = append(*diffs, StructuralDiff{
			Path:          path,
			MismatchKind:  "Nil-Vs-NonNil-Mismatch",
			ExpectedType:  describeGoType(expVal),
			ActualType:    describeGoType(actVal),
			ExpectedValue: safeInterface(expVal),
			ActualValue:   safeInterface(actVal),
		})
		return
	}

	if actVal.Type() != expVal.Type() {
		actStr := fmt.Sprintf("%v", safeInterface(actVal))
		expStr := fmt.Sprintf("%v", safeInterface(expVal))
		looksIdentical := actStr == expStr
		warning := ""
		kind := "Type-Mismatch"
		if looksIdentical {
			kind = "Type-Coercion-Trap (Identical fmt.Sprint Representation)"
			warning = fmt.Sprintf(
				"AWAS KEBUTAAN LOG: Nilai aktual (%v) dan ekspektasi (%v) tampak identik di fmt.Println, tetapi berbeda tipe (%s vs %s).",
				actStr, expStr, describeGoType(actVal), describeGoType(expVal),
			)
		}
		*diffs = append(*diffs, StructuralDiff{
			Path:             path,
			MismatchKind:     kind,
			ExpectedType:     describeGoType(expVal),
			ActualType:       describeGoType(actVal),
			ExpectedValue:    safeInterface(expVal),
			ActualValue:      safeInterface(actVal),
			AmbiguityWarning: warning,
		})
		return
	}

	switch actVal.Kind() {
	case reflect.Float32, reflect.Float64:
		af := actVal.Float()
		ef := expVal.Float()
		if math.IsNaN(af) || math.IsNaN(ef) {
			*diffs = append(*diffs, StructuralDiff{
				Path:             path,
				MismatchKind:     "NaN-Contamination",
				ExpectedType:     describeGoType(expVal),
				ActualType:       describeGoType(actVal),
				ExpectedValue:    safeInterface(expVal),
				ActualValue:      safeInterface(actVal),
				AmbiguityWarning: "Operasi float menghasilkan NaN (periksa pembagian 0.0/0.0 atau propagasi input NaN).",
			})
			return
		}
		if af != ef {
			delta := af - ef
			absDelta := math.Abs(delta)
			roundedDelta := math.Round(delta*1e8) / 1e8
			kind := "Numeric-Value-Mismatch"
			warning := ""
			if absDelta > 0 && absDelta < 0.01 {
				kind = "Rounding-Or-Floating-Point-Drift"
				warning = "Selisih pecahan kecil (< 0.01) terdeteksi; gunakan pembulatan deterministik (math.Round(val*100)/100) di setiap tahap."
			}
			*diffs = append(*diffs, StructuralDiff{
				Path:             path,
				MismatchKind:     kind,
				ExpectedType:     describeGoType(expVal),
				ActualType:       describeGoType(actVal),
				ExpectedValue:    ef,
				ActualValue:      af,
				NumericDelta:     &roundedDelta,
				AmbiguityWarning: warning,
			})
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ai := actVal.Int()
		ei := expVal.Int()
		if ai != ei {
			delta := float64(ai - ei)
			*diffs = append(*diffs, StructuralDiff{
				Path:          path,
				MismatchKind:  "Numeric-Value-Mismatch",
				ExpectedType:  describeGoType(expVal),
				ActualType:    describeGoType(actVal),
				ExpectedValue: ei,
				ActualValue:   ai,
				NumericDelta:  &delta,
			})
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		au := actVal.Uint()
		eu := expVal.Uint()
		if au != eu {
			delta := float64(au) - float64(eu)
			*diffs = append(*diffs, StructuralDiff{
				Path:          path,
				MismatchKind:  "Numeric-Value-Mismatch",
				ExpectedType:  describeGoType(expVal),
				ActualType:    describeGoType(actVal),
				ExpectedValue: eu,
				ActualValue:   au,
				NumericDelta:  &delta,
			})
		}

	case reflect.Slice:
		// Deteksi jebakan klasik Go: nil slice vs empty slice `[]T{}`
		if actVal.IsNil() != expVal.IsNil() && actVal.Len() == 0 && expVal.Len() == 0 {
			*diffs = append(*diffs, StructuralDiff{
				Path:             path,
				MismatchKind:     "Nil-Vs-Empty-Slice-Trap",
				ExpectedType:     describeGoType(expVal),
				ActualType:       describeGoType(actVal),
				ExpectedValue:    safeInterface(expVal),
				ActualValue:      safeInterface(actVal),
				AmbiguityWarning: "AWAS KEBUTAAN LOG GO: Slice aktual dan ekspektasi sama-sama berpanjang 0 (tampak sebagai `[]` di fmt.Println), tetapi salah satunya `nil` dan yang lain `[]T{}` non-nil. Inisialisasi slice kosong secara eksplisit (`make([]T, 0)` atau `[]T{}`).",
			})
			return
		}
		if actVal.Len() != expVal.Len() {
			*diffs = append(*diffs, StructuralDiff{
				Path:          path,
				MismatchKind:  "Slice-Length-Mismatch",
				ExpectedType:  describeGoType(expVal),
				ActualType:    describeGoType(actVal),
				ExpectedValue: safeInterface(expVal),
				ActualValue:   safeInterface(actVal),
			})
			return
		}
		for i := 0; i < actVal.Len(); i++ {
			computeDiffReflect(actVal.Index(i), expVal.Index(i), fmt.Sprintf("%s[%d]", path, i), diffs)
		}

	case reflect.Array:
		for i := 0; i < actVal.Len(); i++ {
			computeDiffReflect(actVal.Index(i), expVal.Index(i), fmt.Sprintf("%s[%d]", path, i), diffs)
		}

	case reflect.Map:
		if actVal.IsNil() != expVal.IsNil() && actVal.Len() == 0 && expVal.Len() == 0 {
			*diffs = append(*diffs, StructuralDiff{
				Path:             path,
				MismatchKind:     "Nil-Vs-Empty-Map-Trap",
				ExpectedType:     describeGoType(expVal),
				ActualType:       describeGoType(actVal),
				ExpectedValue:    safeInterface(expVal),
				ActualValue:      safeInterface(actVal),
				AmbiguityWarning: "AWAS KEBUTAAN LOG GO: Map aktual dan ekspektasi sama-sama kosong, tetapi salah satunya `nil` dan yang lain `map[K]V{}` non-nil.",
			})
			return
		}
		for _, k := range expVal.MapKeys() {
			subPath := fmt.Sprintf("%s[%v]", path, safeInterface(k))
			av := actVal.MapIndex(k)
			ev := expVal.MapIndex(k)
			if !av.IsValid() {
				*diffs = append(*diffs, StructuralDiff{
					Path:          subPath,
					MismatchKind:  "Missing-Map-Key",
					ExpectedType:  describeGoType(ev),
					ActualType:    "missing",
					ExpectedValue: safeInterface(ev),
					ActualValue:   nil,
				})
			} else {
				computeDiffReflect(av, ev, subPath, diffs)
			}
		}
		for _, k := range actVal.MapKeys() {
			ev := expVal.MapIndex(k)
			if !ev.IsValid() {
				av := actVal.MapIndex(k)
				subPath := fmt.Sprintf("%s[%v]", path, safeInterface(k))
				*diffs = append(*diffs, StructuralDiff{
					Path:          subPath,
					MismatchKind:  "Unexpected-Extra-Map-Key",
					ExpectedType:  "absent",
					ActualType:    describeGoType(av),
					ExpectedValue: nil,
					ActualValue:   safeInterface(av),
				})
			}
		}

	case reflect.Struct:
		t := actVal.Type()
		for i := 0; i < actVal.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			subPath := fmt.Sprintf("%s.%s", path, field.Name)
			computeDiffReflect(actVal.Field(i), expVal.Field(i), subPath, diffs)
		}

	case reflect.Ptr:
		if actVal.IsNil() && expVal.IsNil() {
			return
		}
		if actVal.IsNil() || expVal.IsNil() {
			*diffs = append(*diffs, StructuralDiff{
				Path:          path,
				MismatchKind:  "Nil-Pointer-Mismatch",
				ExpectedType:  describeGoType(expVal),
				ActualType:    describeGoType(actVal),
				ExpectedValue: safeInterface(expVal),
				ActualValue:   safeInterface(actVal),
			})
			return
		}
		computeDiffReflect(actVal.Elem(), expVal.Elem(), path, diffs)

	default:
		if !reflect.DeepEqual(safeInterface(actVal), safeInterface(expVal)) {
			*diffs = append(*diffs, StructuralDiff{
				Path:          path,
				MismatchKind:  "Value-Mismatch",
				ExpectedType:  describeGoType(expVal),
				ActualType:    describeGoType(actVal),
				ExpectedValue: safeInterface(expVal),
				ActualValue:   safeInterface(actVal),
			})
		}
	}
}

// ============================================================================
// BAGIAN 3: EKSEKUSI TERISOLASI (PANIC RECOVERY + ERROR + MUTATION TRACKING)
// ============================================================================

type isolatedExecResult[TIn any, TOut any] struct {
	ok                 bool
	panicked           bool
	actual             TOut
	durationMs         float64
	inputSnapshot      TIn
	postExecutionInput TIn
	inputMutated       bool
	mutationDiff       []StructuralDiff
	serializedErr      *SerializedError
	rawErr             error
}

func executeIsolated[TIn any, TOut any](
	subject SubjectFunc[TIn, TOut],
	rawInput TIn,
	enforceImmutability bool,
) (res isolatedExecResult[TIn, TOut]) {
	baselineSnapshot := SafeClone(rawInput)
	workingInput := SafeClone(rawInput)
	res.inputSnapshot = baselineSnapshot

	start := time.Now()
	defer func() {
		elapsed := float64(time.Since(start).Nanoseconds()) / 1e6
		res.durationMs = math.Round(elapsed*10000) / 10000

		if r := recover(); r != nil {
			res.ok = false
			res.panicked = true
			stackLines := strings.Split(string(debug.Stack()), "\n")
			shortStack := strings.Join(stackLines[:minInt(len(stackLines), 8)], "\n")
			res.serializedErr = &SerializedError{
				Kind:    "Panic",
				Type:    fmt.Sprintf("%T", r),
				Message: fmt.Sprintf("%v", r),
				Stack:   shortStack,
			}
			res.postExecutionInput = workingInput
		}
	}()

	actual, err := subject(workingInput)
	elapsed := float64(time.Since(start).Nanoseconds()) / 1e6
	res.durationMs = math.Round(elapsed*10000) / 10000
	res.postExecutionInput = workingInput

	if enforceImmutability {
		mutDiffs := ComputeStructuralDiff(workingInput, baselineSnapshot, "$input")
		if len(mutDiffs) > 0 {
			res.inputMutated = true
			res.mutationDiff = mutDiffs
		}
	}

	if err != nil {
		res.ok = false
		res.rawErr = err
		res.serializedErr = &SerializedError{
			Kind:    "Error",
			Type:    fmt.Sprintf("%T", err),
			Message: err.Error(),
		}
		return res
	}

	res.ok = true
	res.actual = actual
	return res
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func matchesPoisonError[TIn any, TOut any](res isolatedExecResult[TIn, TOut], poison PoisonCase[TIn, TOut]) bool {
	if poison.ErrorMatcher != nil && res.rawErr != nil {
		if !poison.ErrorMatcher(res.rawErr) {
			return false
		}
	}
	if poison.ExpectedErrorContains != "" && res.serializedErr != nil {
		if !strings.Contains(
			strings.ToLower(res.serializedErr.Message),
			strings.ToLower(poison.ExpectedErrorContains),
		) {
			return false
		}
	}
	return true
}

// onlyIdenticalNaN melaporkan apakah seluruh diff semata-mata pasangan NaN-vs-NaN.
// Dipakai pada uji determinisme: run ulang yang mereproduksi NaN yang sama berarti
// hasilnya TIDAK BERUBAH (terkontaminasi, tapi identik), sehingga bukan perilaku
// nondeterministik. Ini tidak mengubah sikap terhadap ekspektasi: NaN vs ekspektasi
// tetap kontaminasi. Nilai "NaN" di sini adalah representasi safeInterface untuk
// float NaN; kind NaN-Contamination hanya muncul dari cabang float.
func onlyIdenticalNaN(diffs []StructuralDiff) bool {
	if len(diffs) == 0 {
		return false
	}
	for _, d := range diffs {
		if d.MismatchKind != "NaN-Contamination" || d.ExpectedValue != "NaN" || d.ActualValue != "NaN" {
			return false
		}
	}
	return true
}

// ============================================================================
// BAGIAN 4: PEMBANGUN KONTEKS DIAGNOSTIK ANTI-KEBUTAAN AGEN
// ============================================================================

func buildAgentDiagnosticContext(report *VerificationReport) AgentDiagnosticContext {
	alerts := make([]BlindSpotAlert, 0)
	directives := make([]RepairDirective, 0)
	classificationCounts := make(map[string]int)
	perspectiveSet := make(map[string]bool)

	for _, f := range report.Failures {
		classificationCounts[f.Classification]++
		perspectiveSet[f.Perspective] = true
	}

	affectedPerspectives := make([]string, 0, len(perspectiveSet))
	for p := range perspectiveSet {
		affectedPerspectives = append(affectedPerspectives, p)
	}

	// 1. Deteksi Ilusi "Hijau Semu" (False-Green Illusion)
	hasSecondaryFailures := false
	for _, f := range report.Failures {
		if f.Perspective != "Primary-Target" {
			hasSecondaryFailures = true
			break
		}
	}
	if report.PrimaryPassed && (hasSecondaryFailures || len(report.UnverifiedPerspectives) > 0) {
		violated := make([]string, 0)
		for _, p := range affectedPerspectives {
			if p != "Primary-Target" {
				violated = append(violated, p)
			}
		}
		violated = append(violated, report.UnverifiedPerspectives...)
		alerts = append(alerts, BlindSpotAlert{
			Code:                 "FALSE_GREEN_ILLUSION",
			Severity:             "CRITICAL",
			ViolatedPerspectives: violated,
			Explanation:          "Gerbang Primary-Target LULUS 100%, namun sistem GAGAL pada perspektif sekunder atau memiliki gerbang wajib yang belum diverifikasi. JANGAN klaim tugas selesai.",
		})
	}

	// 2. Deteksi Panic vs Error (Kebutaan Khas Go)
	if classificationCounts["Uncaught-Panic"] > 0 || classificationCounts["Unexpected-Panic-Instead-Of-Error"] > 0 {
		alerts = append(alerts, BlindSpotAlert{
			Code:        "UNHANDLED_GO_RUNTIME_PANIC",
			Severity:    "CRITICAL",
			Explanation: "Fungsi mengalami runtime panic (mis. nil pointer dereference atau slice index out of range) alih-alih mengembalikan nilai `error` idiomatis Go.",
		})
		directives = append(directives, RepairDirective{
			Priority:    1,
			ActionCode:  "REPLACE_PANIC_WITH_EXPLICIT_ERROR_RETURN",
			Instruction: "Periksa panjang slice (`len(s) == 0`) dan pointer `nil` sebelum mengakses elemen/field, lalu kembalikan `fmt.Errorf(...)` atau sentinel error (bukan panic).",
		})
	}

	// 3. Deteksi Ambiguitas Representasi / Nil-vs-Empty Slice / Floating-Point Drift
	for _, f := range report.Failures {
		for _, diff := range f.StructuralDiff {
			if diff.AmbiguityWarning != "" {
				code := strings.ToUpper(strings.ReplaceAll(diff.MismatchKind, "-", "_"))
				alerts = append(alerts, BlindSpotAlert{
					Code:        code,
					Severity:    "HIGH",
					CaseName:    f.CaseName,
					Path:        diff.Path,
					Explanation: diff.AmbiguityWarning,
				})
			}
		}
	}

	// 4. Deteksi Mutasi Slice/Map/Pointer In-Place
	mutatedPathMap := make(map[string]bool)
	var mutatedPaths []string
	for _, f := range report.Failures {
		if f.Classification == "Input-Mutation-Side-Effect" {
			for _, md := range f.MutationDiff {
				if !mutatedPathMap[md.Path] {
					mutatedPathMap[md.Path] = true
					mutatedPaths = append(mutatedPaths, md.Path)
				}
			}
		}
	}
	if len(mutatedPaths) > 0 {
		alerts = append(alerts, BlindSpotAlert{
			Code:         "SHARED_SLICE_OR_MAP_MUTATION",
			Severity:     "HIGH",
			MutatedPaths: mutatedPaths,
			Explanation:  fmt.Sprintf("Subject memodifikasi elemen slice/map/pointer pada argumen input secara in-place pada path [%s]. Di Go, header slice yang dikirim by-value tetap menunjuk ke backing array milik pemanggil.", strings.Join(mutatedPaths, ", ")),
		})
		directives = append(directives, RepairDirective{
			Priority:    1,
			ActionCode:  "ELIMINATE_INPLACE_MUTATION",
			TargetPaths: mutatedPaths,
			Instruction: "Salin slice/map input terlebih dahulu (mis. `cloned := append([]T(nil), input.Slice...)`) sebelum memanggil `sort.Slice`, `slices.Sort`, atau memodifikasi elemen.",
		})
	}

	// 5. Direktif Penjaga Batas (Boundary Guards)
	if classificationCounts["Missing-Expected-Error"] > 0 || classificationCounts["Wrong-Error-Message-Or-Type"] > 0 {
		directives = append(directives, RepairDirective{
			Priority:    1,
			ActionCode:  "HARDEN_BOUNDARY_GUARDS",
			Instruction: "Tambahkan validasi pre-condition di awal fungsi untuk input nil, angka negatif, atau math.IsNaN, dan kembalikan `error` deskriptif sesuai spesifikasi.",
		})
	}

	// 6. Direktif Keselarasan Transformasi & Oracle
	if classificationCounts["Implementation-Defect"] > 0 || classificationCounts["Behavioral-Drift"] > 0 {
		sampleDiffs := make([]SampleDiffEntry, 0)
		for _, f := range report.Failures {
			if len(f.StructuralDiff) > 0 && len(sampleDiffs) < 3 {
				name := f.CaseName
				if name == "" && f.SampleIndex != nil {
					name = fmt.Sprintf("Oracle-Sample-%d", *f.SampleIndex)
				}
				sampleDiffs = append(sampleDiffs, SampleDiffEntry{
					Case:  name,
					Diffs: f.StructuralDiff,
				})
			}
		}
		directives = append(directives, RepairDirective{
			Priority:    2,
			ActionCode:  "ALIGN_CORE_TRANSFORM_AND_TYPES",
			SampleDiffs: sampleDiffs,
			Instruction: "Periksa `structuralDiff` di atas: pastikan inisialisasi slice kosong (`[]T{}` vs `nil`) dan pembulatan desimal (`math.Round(val*100)/100`) identik dengan spesifikasi dan Reference Oracle.",
		})
	}

	// 7. Direktif Regresi Jalur Tetangga
	if classificationCounts["Secondary-Regression"] > 0 || classificationCounts["Secondary-Regression-Error"] > 0 {
		regIDs := make([]string, 0)
		for _, f := range report.Failures {
			if f.Perspective == "Post-PASS-Regression" && f.RegressionID != "" {
				regIDs = append(regIDs, f.RegressionID)
			}
		}
		alerts = append(alerts, BlindSpotAlert{
			Code:          "NEIGHBOR_PATH_REGRESSION",
			Severity:      "CRITICAL",
			RegressionIDs: regIDs,
			Explanation:   fmt.Sprintf("Perubahan terbaru merusak jalur fungsional tetangga yang sebelumnya sudah stabil (%s).", strings.Join(regIDs, ", ")),
		})
		directives = append(directives, RepairDirective{
			Priority:      1,
			ActionCode:    "RESTORE_REGRESSION_INVARIANTS",
			RegressionIDs: regIDs,
			Instruction:   fmt.Sprintf("Jangan menimpa cabang logika untuk kasus khusus/legacy (%s) saat memperbaiki kasus primer. Isolasi kondisi cabang dengan predikat yang spesifik.", strings.Join(regIDs, ", ")),
		})
	}

	// 8. Penentuan Blast Radius & Next Action Directive
	blastRadius := "NONE"
	if len(report.Failures) > 0 {
		if len(affectedPerspectives) >= 3 {
			blastRadius = "SYSTEMIC_MULTI_PERSPECTIVE"
		} else if perspectiveSet["Post-PASS-Regression"] {
			blastRadius = "CROSS_PATH_REGRESSION"
		} else {
			blastRadius = "LOCALIZED_DEFECT"
		}
	}

	nextActionDirective := "READY_TO_COMMIT"
	if !report.IsCompleteAndVerified {
		switch {
		case !report.PrimaryPassed:
			nextActionDirective = "FIX_PRIMARY_SPECIFICATION_FIRST"
		case classificationCounts["Secondary-Regression"] > 0 || classificationCounts["Secondary-Regression-Error"] > 0:
			nextActionDirective = "REVERT_OR_ISOLATE_BREAKING_BRANCH"
		case classificationCounts["Uncaught-Panic"] > 0 || classificationCounts["Unexpected-Panic-Instead-Of-Error"] > 0:
			nextActionDirective = "REPLACE_PANIC_WITH_ERROR_RETURN"
		case classificationCounts["Missing-Expected-Error"] > 0 || classificationCounts["Unhandled-Edge-Case"] > 0:
			nextActionDirective = "PATCH_BOUNDARY_AND_POISON_GUARDS"
		case classificationCounts["Behavioral-Drift"] > 0:
			nextActionDirective = "RECONCILE_ORACLE_PRECISION_DRIFT"
		case classificationCounts["Input-Mutation-Side-Effect"] > 0:
			nextActionDirective = "REMOVE_INPLACE_INPUT_MUTATION"
		case len(report.UnverifiedPerspectives) > 0:
			nextActionDirective = "POPULATE_VACUOUS_VERIFICATION_GATES"
		default:
			nextActionDirective = "INVESTIGATE_FAILURE_EVIDENCE"
		}
	}

	return AgentDiagnosticContext{
		NextActionDirective:   nextActionDirective,
		BlastRadius:           blastRadius,
		FalseGreenDetected:    report.PrimaryPassed && !report.IsCompleteAndVerified,
		ClassificationSummary: classificationCounts,
		BlindSpotAlerts:       alerts,
		RepairDirectives:      directives,
	}
}

// ============================================================================
// BAGIAN 5: FUNGSI UTAMA `RunAgentVerificationSuite` (4 GERBANG VERIFIKASI)
// ============================================================================

// RunAgentVerificationSuite mengeksekusi pengujian 4 perspektif secara deterministik.
func RunAgentVerificationSuite[TIn any, TOut any](suiteSpec SuiteSpec[TIn, TOut]) VerificationReport {
	if suiteSpec.Subject == nil {
		panic("suiteSpec.Subject tidak boleh nil")
	}

	comparator := suiteSpec.Comparator
	if comparator == nil {
		comparator = DefaultComparator[TOut]
	}

	opts := DefaultSuiteOptions()
	if suiteSpec.Options != nil {
		opts = *suiteSpec.Options
		if opts.DeterminismRuns <= 0 {
			opts.DeterminismRuns = 2
		}
		if opts.MaxOracleFailures <= 0 {
			opts.MaxOracleFailures = 10
		}
	}

	moduleName := suiteSpec.Name
	if moduleName == "" {
		moduleName = "Unnamed-Go-Module"
	}
	domain := suiteSpec.Domain
	if domain == "" {
		domain = "general-app"
	}

	diffCount := 0
	if suiteSpec.ReferenceOracle != nil {
		diffCount = len(suiteSpec.GeneratedSamples)
	}

	report := VerificationReport{
		Module:                        moduleName,
		Domain:                        domain,
		PrimaryPassed:                 false,
		PerspectivesChecked:           make([]string, 0),
		UnverifiedPerspectives:        make([]string, 0),
		Failures:                      make([]FailureEvidence, 0),
		GeneratedRegressionCandidates: make([]RegressionCandidate, 0),
		Metrics: SuiteMetrics{
			GateCounts: GateCounts{
				Primary:      len(suiteSpec.PrimaryCases),
				Poison:       len(suiteSpec.PoisonInputs),
				Differential: diffCount,
				Regression:   len(suiteSpec.RegressionSuite),
			},
		},
	}

	recordMetrics := func(durationMs float64) {
		report.Metrics.TotalCasesExecuted++
		report.Metrics.TotalDurationMs = math.Round((report.Metrics.TotalDurationMs+durationMs)*10000) / 10000
		if durationMs > report.Metrics.MaxCaseDurationMs {
			report.Metrics.MaxCaseDurationMs = durationMs
		}
	}

	checkCommonInvariants := func(res isolatedExecResult[TIn, TOut], perspective, caseName string, caseMaxMs float64) {
		if res.inputMutated {
			report.Failures = append(report.Failures, FailureEvidence{
				Perspective:    perspective,
				Classification: "Input-Mutation-Side-Effect",
				CaseName:       caseName,
				Input:          res.inputSnapshot,
				MutatedInput:   res.postExecutionInput,
				MutationDiff:   res.mutationDiff,
				Detail:         "Subject memodifikasi elemen slice/map/pointer pada argumen input secara in-place.",
			})
		}

		budgetMs := opts.MaxDurationMs
		if caseMaxMs > 0 {
			budgetMs = caseMaxMs
		}
		if budgetMs > 0 && res.durationMs > budgetMs {
			report.Failures = append(report.Failures, FailureEvidence{
				Perspective:    perspective,
				Classification: "Performance-Bottleneck",
				CaseName:       caseName,
				Input:          res.inputSnapshot,
				DurationMs:     res.durationMs,
				BudgetMs:       budgetMs,
				Detail:         fmt.Sprintf("Durasi eksekusi (%.4fms) melampaui batas (%.4fms).", res.durationMs, budgetMs),
			})
		}
	}

	// ------------------------------------------------------------------------
	// 1. GERBANG PRIMER: Spesifikasi Utama & Determinisme
	// ------------------------------------------------------------------------
	if len(suiteSpec.PrimaryCases) > 0 {
		primaryGateFailures := 0

		for _, tc := range suiteSpec.PrimaryCases {
			enforceImm := opts.EnforceImmutability
			if tc.EnforceImmutability != nil {
				enforceImm = *tc.EnforceImmutability
			}

			res := executeIsolated(suiteSpec.Subject, tc.Input, enforceImm)
			recordMetrics(res.durationMs)

			if !res.ok {
				primaryGateFailures++
				classification := "Unexpected-Error"
				if res.panicked {
					classification = "Uncaught-Panic"
				}
				report.Failures = append(report.Failures, FailureEvidence{
					Perspective:    "Primary-Target",
					Classification: classification,
					CaseName:       tc.Name,
					Input:          res.inputSnapshot,
					Expected:       tc.Expected,
					Error:          res.serializedErr,
					DurationMs:     res.durationMs,
				})
				continue
			}

			if !comparator(res.actual, tc.Expected) {
				primaryGateFailures++
				report.Failures = append(report.Failures, FailureEvidence{
					Perspective:    "Primary-Target",
					Classification: "Implementation-Defect",
					CaseName:       tc.Name,
					Input:          res.inputSnapshot,
					Expected:       tc.Expected,
					Actual:         res.actual,
					StructuralDiff: ComputeStructuralDiff(res.actual, tc.Expected, "$"),
					DurationMs:     res.durationMs,
				})
			}

			// Uji Determinisme / Idempotensi (menangkap map iteration randomness atau state global bocor)
			for run := 1; run < opts.DeterminismRuns; run++ {
				repeatRes := executeIsolated(suiteSpec.Subject, tc.Input, false)
				recordMetrics(repeatRes.durationMs)

				if !repeatRes.ok || !comparator(repeatRes.actual, res.actual) {
					var sDiff []StructuralDiff
					if repeatRes.ok {
						sDiff = ComputeStructuralDiff(repeatRes.actual, res.actual, "$")
					}
					// Hasil ulang yang identik-terkontaminasi (NaN-vs-NaN) BUKAN
					// nondeterminisme: defect-nya sudah dicatat pada run pertama
					// (Implementation-Defect), jangan gandakan dengan label yang
					// menyesatkan. NaN tetap kontaminasi terhadap ekspektasi.
					if repeatRes.ok && onlyIdenticalNaN(sDiff) {
						break
					}
					primaryGateFailures++
					report.Failures = append(report.Failures, FailureEvidence{
						Perspective:     "Primary-Target",
						Classification:  "Nondeterministic-Behavior",
						CaseName:        tc.Name,
						Input:           res.inputSnapshot,
						FirstRunActual:  res.actual,
						RepeatRunActual: repeatRes.actual,
						StructuralDiff:  sDiff,
						Error:           repeatRes.serializedErr,
						Detail:          fmt.Sprintf("Hasil berubah pada eksekusi ulang ke-%d (waspadai iterasi map Go yang acak atau state global).", run+1),
					})
					break
				}
			}

			checkCommonInvariants(res, "Primary-Target", tc.Name, tc.MaxDurationMs)
		}

		report.PerspectivesChecked = append(report.PerspectivesChecked, "Primary-Target")
		report.PrimaryPassed = primaryGateFailures == 0
	} else {
		report.UnverifiedPerspectives = append(report.UnverifiedPerspectives, "Primary-Target")
	}

	// ------------------------------------------------------------------------
	// 2. GERBANG PERSPEKTIF BATAS & RACUN (Poison & Boundary)
	// ------------------------------------------------------------------------
	if len(suiteSpec.PoisonInputs) > 0 {
		for _, poison := range suiteSpec.PoisonInputs {
			enforceImm := opts.EnforceImmutability
			if poison.EnforceImmutability != nil {
				enforceImm = *poison.EnforceImmutability
			}

			res := executeIsolated(suiteSpec.Subject, poison.Input, enforceImm)
			recordMetrics(res.durationMs)

			if !res.ok {
				// Jika subject mengalami PANIC padahal tidak diizinkan panic
				if res.panicked && !poison.AllowPanic {
					classification := "Uncaught-Panic"
					detail := poison.FailureHint
					if poison.ExpectError {
						classification = "Unexpected-Panic-Instead-Of-Error"
						detail = "Subject seharusnya mengembalikan nilai `error` secara idiomatis, tetapi justru mengalami runtime panic."
					} else if detail == "" {
						detail = "Subject mengalami runtime panic yang tidak tertangani."
					}
					report.Failures = append(report.Failures, FailureEvidence{
						Perspective:    "Boundary-And-Poison",
						Classification: classification,
						CaseName:       poison.Name,
						Input:          res.inputSnapshot,
						Detail:         detail,
						Error:          res.serializedErr,
						DurationMs:     res.durationMs,
					})
					continue
				}

				if !poison.ExpectError {
					detail := poison.FailureHint
					if detail == "" {
						detail = "Subject mengembalikan error yang tidak diharapkan pada kasus batas."
					}
					report.Failures = append(report.Failures, FailureEvidence{
						Perspective:    "Boundary-And-Poison",
						Classification: "Unexpected-Error",
						CaseName:       poison.Name,
						Input:          res.inputSnapshot,
						Detail:         detail,
						Error:          res.serializedErr,
						DurationMs:     res.durationMs,
					})
				} else if !matchesPoisonError(res, poison) {
					report.Failures = append(report.Failures, FailureEvidence{
						Perspective:    "Boundary-And-Poison",
						Classification: "Wrong-Error-Message-Or-Type",
						CaseName:       poison.Name,
						Input:          res.inputSnapshot,
						Detail:         fmt.Sprintf("Subject mengembalikan error, tetapi tidak memuat pola %q atau gagal pada ErrorMatcher.", poison.ExpectedErrorContains),
						Error:          res.serializedErr,
						DurationMs:     res.durationMs,
					})
				}
				continue
			}

			if poison.ExpectError {
				detail := poison.FailureHint
				if detail == "" {
					detail = "Subject seharusnya menolak input racun dengan mengembalikan `error`, namun berhasil lolos (err == nil)."
				}
				report.Failures = append(report.Failures, FailureEvidence{
					Perspective:    "Boundary-And-Poison",
					Classification: "Missing-Expected-Error",
					CaseName:       poison.Name,
					Input:          res.inputSnapshot,
					Actual:         res.actual,
					Detail:         detail,
					DurationMs:     res.durationMs,
				})
				continue
			}

			if poison.InvariantCheck != nil {
				invOk := false
				var invPanic any
				func() {
					defer func() {
						if r := recover(); r != nil {
							invPanic = r
						}
					}()
					invOk = poison.InvariantCheck(res.actual, SafeClone(poison.Input))
				}()

				if invPanic != nil {
					report.Failures = append(report.Failures, FailureEvidence{
						Perspective:    "Boundary-And-Poison",
						Classification: "Verifier-Defect",
						CaseName:       poison.Name,
						Input:          res.inputSnapshot,
						Detail:         "Fungsi InvariantCheck mengalami panic saat memeriksa output.",
						Error: &SerializedError{
							Kind:    "Panic",
							Type:    fmt.Sprintf("%T", invPanic),
							Message: fmt.Sprintf("%v", invPanic),
						},
					})
				} else if !invOk {
					detail := poison.FailureHint
					if detail == "" {
						detail = "Invarian batas dilanggar."
					}
					report.Failures = append(report.Failures, FailureEvidence{
						Perspective:    "Boundary-And-Poison",
						Classification: "Unhandled-Edge-Case",
						CaseName:       poison.Name,
						Input:          res.inputSnapshot,
						Actual:         res.actual,
						Detail:         detail,
						DurationMs:     res.durationMs,
					})
				}
			}

			checkCommonInvariants(res, "Boundary-And-Poison", poison.Name, poison.MaxDurationMs)
		}
		report.PerspectivesChecked = append(report.PerspectivesChecked, "Boundary-And-Poison")
	} else {
		report.UnverifiedPerspectives = append(report.UnverifiedPerspectives, "Boundary-And-Poison")
	}

	// ------------------------------------------------------------------------
	// 3. GERBANG EKUIVALENSI ORACLE (Differential Equivalence)
	// ------------------------------------------------------------------------
	if suiteSpec.ReferenceOracle != nil && len(suiteSpec.GeneratedSamples) > 0 {
		oracleDriftCount := 0

		for i, sample := range suiteSpec.GeneratedSamples {
			idxCopy := i
			oracleRes := executeIsolated(suiteSpec.ReferenceOracle, sample, false)
			subjectRes := executeIsolated(suiteSpec.Subject, sample, opts.EnforceImmutability)
			recordMetrics(subjectRes.durationMs)

			if oracleRes.ok != subjectRes.ok {
				oracleDriftCount++
				var expVal, actVal any
				if oracleRes.ok {
					expVal = oracleRes.actual
				} else {
					expVal = oracleRes.serializedErr
				}
				if subjectRes.ok {
					actVal = subjectRes.actual
				} else {
					actVal = subjectRes.serializedErr
				}
				report.Failures = append(report.Failures, FailureEvidence{
					Perspective:    "Differential-Equivalence",
					Classification: "Behavioral-Drift-Error-Mismatch",
					SampleIndex:    &idxCopy,
					CaseName:       fmt.Sprintf("Oracle-Sample-%d", i),
					Input:          subjectRes.inputSnapshot,
					Expected:       expVal,
					Actual:         actVal,
					DurationMs:     subjectRes.durationMs,
				})
			} else if oracleRes.ok && !comparator(subjectRes.actual, oracleRes.actual) {
				oracleDriftCount++
				report.Failures = append(report.Failures, FailureEvidence{
					Perspective:    "Differential-Equivalence",
					Classification: "Behavioral-Drift",
					SampleIndex:    &idxCopy,
					CaseName:       fmt.Sprintf("Oracle-Sample-%d", i),
					Input:          subjectRes.inputSnapshot,
					Expected:       oracleRes.actual,
					Actual:         subjectRes.actual,
					StructuralDiff: ComputeStructuralDiff(subjectRes.actual, oracleRes.actual, "$"),
					DurationMs:     subjectRes.durationMs,
				})
			}

			checkCommonInvariants(subjectRes, "Differential-Equivalence", fmt.Sprintf("Oracle-Sample-%d", i), opts.MaxDurationMs)

			if oracleDriftCount > 0 && (opts.StopOnFirstOracleDrift || oracleDriftCount >= opts.MaxOracleFailures) {
				break
			}
		}
		report.PerspectivesChecked = append(report.PerspectivesChecked, "Differential-Equivalence")
	} else if domain == "low-level" || suiteSpec.ReferenceOracle != nil {
		report.UnverifiedPerspectives = append(report.UnverifiedPerspectives, "Differential-Equivalence")
	}

	// ------------------------------------------------------------------------
	// 4. GERBANG POST-PASS & REGRESI SEKUNDER
	// ------------------------------------------------------------------------
	if len(suiteSpec.RegressionSuite) > 0 {
		for _, regCase := range suiteSpec.RegressionSuite {
			enforceImm := opts.EnforceImmutability
			if regCase.EnforceImmutability != nil {
				enforceImm = *regCase.EnforceImmutability
			}

			res := executeIsolated(suiteSpec.Subject, regCase.Input, enforceImm)
			recordMetrics(res.durationMs)

			caseName := regCase.Name
			if caseName == "" {
				caseName = regCase.ID
			}

			if !res.ok {
				report.Failures = append(report.Failures, FailureEvidence{
					Perspective:    "Post-PASS-Regression",
					Classification: "Secondary-Regression-Error",
					RegressionID:   regCase.ID,
					CaseName:       caseName,
					Input:          res.inputSnapshot,
					Expected:       regCase.Expected,
					Error:          res.serializedErr,
					DurationMs:     res.durationMs,
				})
				continue
			}

			if !comparator(res.actual, regCase.Expected) {
				report.Failures = append(report.Failures, FailureEvidence{
					Perspective:    "Post-PASS-Regression",
					Classification: "Secondary-Regression",
					RegressionID:   regCase.ID,
					CaseName:       caseName,
					Input:          res.inputSnapshot,
					Expected:       regCase.Expected,
					Actual:         res.actual,
					StructuralDiff: ComputeStructuralDiff(res.actual, regCase.Expected, "$"),
					DurationMs:     res.durationMs,
				})
			}

			checkCommonInvariants(res, "Post-PASS-Regression", caseName, regCase.MaxDurationMs)
		}
		report.PerspectivesChecked = append(report.PerspectivesChecked, "Post-PASS-Regression")
	} else {
		report.UnverifiedPerspectives = append(report.UnverifiedPerspectives, "Post-PASS-Regression")
	}

	// ------------------------------------------------------------------------
	// 5. EKSTRAKSI KANDIDAT REGRESI & EVALUASI PENYELESAIAN KETAT
	// ------------------------------------------------------------------------
	candIdx := 0
	for _, f := range report.Failures {
		if f.Perspective != "Post-PASS-Regression" && f.Input != nil {
			candIdx++
			name := f.CaseName
			if name == "" {
				name = fmt.Sprintf("Sample-%d", candIdx)
			}
			report.GeneratedRegressionCandidates = append(report.GeneratedRegressionCandidates, RegressionCandidate{
				ID:                fmt.Sprintf("AUTO-REG-%s-%d", strings.ToUpper(f.Perspective), candIdx),
				SourcePerspective: f.Perspective,
				Classification:    f.Classification,
				Name:              name,
				Input:             f.Input,
				Expected:          f.Expected,
				CapturedError:     f.Error,
			})
		}
	}

	requiredGates := []string{"Primary-Target", "Boundary-And-Poison", "Post-PASS-Regression"}
	if domain == "low-level" || suiteSpec.ReferenceOracle != nil {
		requiredGates = append(requiredGates, "Differential-Equivalence")
	}

	hasGate := func(gate string) bool {
		for _, g := range report.PerspectivesChecked {
			if g == gate {
				return true
			}
		}
		return false
	}

	allRequiredGatesChecked := true
	if opts.RequireAllPerspectives {
		for _, req := range requiredGates {
			if !hasGate(req) {
				allRequiredGatesChecked = false
				break
			}
		}
	} else {
		allRequiredGatesChecked = hasGate("Primary-Target")
	}

	report.IsCompleteAndVerified = report.PrimaryPassed && len(report.Failures) == 0 && allRequiredGatesChecked
	report.AgentDiagnosticContext = buildAgentDiagnosticContext(&report)

	return report
}

// PrintVerificationReport mencetak ringkasan laporan dan mengembalikan kode keluar (0 = lulus, 1 = gagal).
func PrintVerificationReport(report VerificationReport) int {
	fmt.Println("====================================================================")
	fmt.Printf("LAPORAN VERIFIKASI (GO): %s\n", report.Module)
	fmt.Println("====================================================================")
	fmt.Printf("- Primary Passed         : %v\n", report.PrimaryPassed)
	fmt.Printf("- Complete & Verified    : %v\n", report.IsCompleteAndVerified)
	fmt.Printf("- Perspectives Checked   : [%s]\n", strings.Join(report.PerspectivesChecked, ", "))
	unverified := "Tidak Ada"
	if len(report.UnverifiedPerspectives) > 0 {
		unverified = strings.Join(report.UnverifiedPerspectives, ", ")
	}
	fmt.Printf("- Unverified Gates       : [%s]\n", unverified)
	fmt.Printf("- Total Cases Executed   : %d (%.4f ms)\n", report.Metrics.TotalCasesExecuted, report.Metrics.TotalDurationMs)
	fmt.Printf("- Next Action Directive  : %s\n", report.AgentDiagnosticContext.NextActionDirective)
	fmt.Printf("- Blast Radius           : %s\n", report.AgentDiagnosticContext.BlastRadius)

	if !report.IsCompleteAndVerified {
		fmt.Println("\n[!] VERIFIKASI GAGAL — BACA KONTEKS DIAGNOSTIK AGEN DI BAWAH INI:")
		ctxBytes, _ := json.MarshalIndent(report.AgentDiagnosticContext, "", "  ")
		fmt.Println(string(ctxBytes))

		if len(report.GeneratedRegressionCandidates) > 0 {
			fmt.Println("\n[+] KANDIDAT REGRESI BARU (Salin ke `suiteSpec.RegressionSuite`):")
			candBytes, _ := json.MarshalIndent(report.GeneratedRegressionCandidates, "", "  ")
			fmt.Println(string(candBytes))
		}
		return 1
	}

	fmt.Println("\n[OK] SELURUH 4 PERSPEKTIF LULUS TANPA REGRESI. SIAP DI-COMMIT.")
	return 0
}
