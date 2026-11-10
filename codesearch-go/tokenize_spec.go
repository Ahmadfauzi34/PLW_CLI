//go:build verify

// ============================================================================
// Penempatan skill Go verification-harness di codesearch, untuk tokenize
// ============================================================================
// - harness.go di direktori ini adalah SALINAN VERBATIM dari skill
//   local-verification-harness-go (core TIDAK diubah; yang dicustom
//   hanya file spec ini, sesuai aturan skill).
// - File ini memakai build tag "verify": build/test/vet normal
//   (tanpa tag) mengabaikannya, sehingga tidak ada main() ganda dengan
//   main.go. Jalankan HANYA lewat ./verify-tokenize.sh, yang memakai
//   `go run` dengan daftar file eksplisit (mode file mengabaikan tag)
//   dan TIDAK mencantumkan main.go — jadi main() di spec ini adalah
//   satu-satunya entry point, sementara tipe options tersedia lewat
//   options.go (dipindah verbatim dari main.go, tanpa perubahan perilaku).
// - Subjek: tokenize (search.go:61). Perilaku yang dikunci diukur dari
//   kode, bukan ditebak: pecah separator non-huruf/non-angka,
//   lowercasing, TANPA split camelCase; input tanpa token -> nil.
// - tokenize tidak punya jalur error, jadi gerbang poison di sini
//   menguji batas nil/separator-only, bukan ExpectError.

package main

import (
	"os"
	"strings"
	"unicode"
)

// STEP 1: Subjek — adaptor tipis ke tokenize, tanpa mengubahnya.
func TokenizeSubject(in string) ([]string, error) {
	return tokenize(in), nil
}

// STEP 2: Reference oracle (independen, lihat catatan di atas).
func TokenizeOracle(in string) ([]string, error) {
	var out []string
	for _, f := range strings.FieldsFunc(in, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r))
	}) {
		out = append(out, strings.ToLower(f))
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// STEP 3: SuiteSpec 4 gerbang.
func BuildSuiteSpec() SuiteSpec[string, []string] {
	opts := DefaultSuiteOptions()

	return SuiteSpec[string, []string]{
		Name:            "CodesearchTokenize",
		Domain:          "low-level",
		Subject:         TokenizeSubject,
		ReferenceOracle: TokenizeOracle,
		Options:         &opts,

		PrimaryCases: []PrimaryCase[string, []string]{
			{
				Name:     "Pecah pada koma dan spasi, lowercasing",
				Input:    "Hello, World!",
				Expected: []string{"hello", "world"},
			},
			{
				Name:     "Underscore dan strip adalah separator",
				Input:    "foo_bar-baz",
				Expected: []string{"foo", "bar", "baz"},
			},
			{
				Name:     "CamelCase TIDAK dipecah oleh tokenize",
				Input:    "getGraphParametersFlat",
				Expected: []string{"getgraphparametersflat"},
			},
			{
				Name:     "Huruf dan angka dalam satu token dipertahankan",
				Input:    "ABC123",
				Expected: []string{"abc123"},
			},
		},

		PoisonInputs: []PoisonCase[string, []string]{
			{
				Name:        "String kosong mengembalikan nil, bukan error",
				Input:       "",
				ExpectError: false,
				InvariantCheck: func(actual []string, _ string) bool {
					return actual == nil
				},
				FailureHint: "tokenize(\"\") harus nil (len 0), bukan error dan bukan slice kosong non-nil.",
			},
			{
				Name:        "Whitespace-only mengembalikan nil",
				Input:       "   ",
				ExpectError: false,
				InvariantCheck: func(actual []string, _ string) bool {
					return actual == nil
				},
				FailureHint: "Input tanpa token harus nil.",
			},
			{
				Name:        "Separator-only mengembalikan nil",
				Input:       "!!!",
				ExpectError: false,
				InvariantCheck: func(actual []string, _ string) bool {
					return actual == nil
				},
				FailureHint: "Input tanpa token harus nil.",
			},
		},

		GeneratedSamples: []string{
			"Hello, World!",
			"foo_bar-baz",
			"a1_b2-c3",
			"hello-world_foo",
			"XMLHttpRequest",
			"Café",
		},

		RegressionSuite: []RegressionCase[string, []string]{
			{
				ID:       "REG-TOKENIZE-NIL-01",
				Name:     "Input tanpa token tetap nil (jebakan nil vs empty slice)",
				Input:    "",
				Expected: nil,
			},
			{
				ID:       "REG-TOKENIZE-CAMEL-02",
				Name:     "CamelCase tetap satu token (jangan tambah splitCamel ke tokenize)",
				Input:    "getGraphParametersFlat",
				Expected: []string{"getgraphparametersflat"},
			},
			{
				ID:       "REG-TOKENIZE-SEPARATOR-03",
				Name:     "Underscore dan strip tetap separator",
				Input:    "foo_bar-baz",
				Expected: []string{"foo", "bar", "baz"},
			},
		},
	}
}

// STEP 4: Runner (sama seperti template).
func main() {
	spec := BuildSuiteSpec()
	report := RunAgentVerificationSuite(spec)
	exitCode := PrintVerificationReport(report)
	os.Exit(exitCode)
}
