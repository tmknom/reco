package configuration

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// File は、設定宣言を読み込む設定ファイル。
type File struct {
	// Dir は設定ファイルのあるディレクトリ、Name はその中のファイル名。
	// fs.FS の名前は先頭の / や .. を含められないため、パスをディレクトリと名前に分けて受ける。
	Dir  fs.FS
	Name string
	// Path は利用者が指定したままの設定ファイルのパス。表示とエラー文言に使う。
	Path string
}

// Load は f を読み、設定宣言に変換する。入力検証に失敗したら設定宣言を返さない。
// エラーに Name ではなく Path を付けるのは、fs.FS の名前が利用者の指定したパスと異なるため。
func (f File) Load() (Declaration, error) {
	d, err := f.read()
	if err != nil {
		return Declaration{}, fmt.Errorf("load configuration %s: %w", f.Path, err)
	}
	return d, nil
}

func (f File) read() (Declaration, error) {
	data, err := fs.ReadFile(f.Dir, f.Name)
	if err != nil {
		return Declaration{}, err
	}
	return parse(data)
}

func parse(data []byte) (Declaration, error) {
	doc, err := decodeDocument(data)
	if err != nil {
		return Declaration{}, err
	}
	return doc.declaration()
}

// yaml.Unmarshal は複数文書の 2 文書目以降を黙って捨てるため、Decoder で 2 文書目の有無を確かめる。
func decodeDocument(data []byte) (document, error) {
	// ライブラリは UTF-16 を UTF-8 に変換した後の位置を返し、ソースの文字と食い違うため、UTF-8 に限る
	if !utf8.Valid(data) {
		return document{}, errors.New("must be encoded in UTF-8")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return document{}, errors.New("no YAML document")
		}
		return document{}, err
	}
	var next yaml.Node
	if err := dec.Decode(&next); !errors.Is(err, io.EOF) {
		if err != nil {
			return document{}, err
		}
		return document{}, errors.New("multiple YAML documents are not allowed")
	}
	return document{root: doc.Content[0], lines: splitLines(data)}, nil
}

// splitLines は、ライブラリが Node.Line と Node.Column を数えるのと同じ規則でソースを行に分ける。
// 改行は CR LF、CR、LF、NEL、LS、PS で、列は文字単位で数え、先頭の BOM は数えない。
func splitLines(data []byte) [][]rune {
	src := []rune(strings.TrimPrefix(string(data), "\uFEFF"))
	var lines [][]rune
	start := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '\r', '\n', '\u0085', '\u2028', '\u2029':
			lines = append(lines, src[start:i])
			if src[i] == '\r' && i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	return append(lines, src[start:])
}
