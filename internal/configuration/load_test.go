package configuration

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/repository"
)

//trace:checks FR-001
func TestDecodeDocument(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    string
		wantErr bool
		// errContains は、このパッケージが決める文言のときだけ書く。構文エラーの文言はライブラリが決める。
		errContains string
	}{
		{
			name: "single document with explicit start and end markers is accepted",
			src:  "---\nx\n...\n",
			want: "x",
		},
		{name: "empty file is rejected as no document", src: "", wantErr: true, errContains: "no YAML document"},
		{name: "comment-only file is rejected as no document", src: "# comment\n", wantErr: true, errContains: "no YAML document"},
		{name: "UTF-16 encoded file is rejected", src: "\xff\xfe" + "x\x00\n\x00", wantErr: true, errContains: "must be encoded in UTF-8"},
		{name: "multiple documents are rejected", src: "x\n---\ny\n", wantErr: true, errContains: "multiple YAML documents"},
		{name: "syntax error in second document is rejected", src: "x\n---\nb: [\n", wantErr: true},
		{name: "syntax error is rejected", src: "a:\n\t- b\n", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeDocument([]byte(tc.src))
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("decodeDocument() error = %v, want it to contain %q", err, tc.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeDocument() error = %v", err)
			}
			if got.root.Value != tc.want {
				t.Errorf("decodeDocument() root = %q, want %q", got.root.Value, tc.want)
			}
		})
	}
}

// TestLoad は、ファイルの読み取りから設定宣言までのつなぎ目を確かめる。各規則は段階ごとのテストが確かめる。
//
//trace:checks FR-001
func TestLoad(t *testing.T) {
	const name = "reco.yaml"
	load := func(src string) (Declaration, error) {
		return Load(fstest.MapFS{name: {Data: []byte(src)}}, name)
	}

	t.Run("valid file is read as a declaration", func(t *testing.T) {
		got, err := load(minimal)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		want := Declaration{Repositories: []RepositoryDeclaration{{
			Reference: repository.Reference{Owner: "owner", Repo: "repo"},
			General:   General{AllowAutoMerge: new(true)},
		}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Load() = %+v, want %+v", got, want)
		}
	})

	t.Run("invalid document is rejected with the file name and no declaration", func(t *testing.T) {
		got, err := load("")
		if err == nil || !strings.HasPrefix(err.Error(), name+": ") {
			t.Errorf("Load() error = %v, want it to start with %q", err, name+": ")
		}
		if !reflect.DeepEqual(got, Declaration{}) {
			t.Errorf("Load() returned a declaration with error: %+v", got)
		}
	})

	t.Run("invalid declaration is rejected with the file name and no declaration", func(t *testing.T) {
		got, err := load("repositories: []\n")
		if err == nil || !strings.HasPrefix(err.Error(), name+": repositories") {
			t.Errorf("Load() error = %v, want it to start with %q", err, name+": repositories")
		}
		if !reflect.DeepEqual(got, Declaration{}) {
			t.Errorf("Load() returned a declaration with error: %+v", got)
		}
	})

	t.Run("missing file is rejected as fs.ErrNotExist", func(t *testing.T) {
		_, err := Load(fstest.MapFS{}, name)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Load() error = %v, want fs.ErrNotExist", err)
		}
	})
}
