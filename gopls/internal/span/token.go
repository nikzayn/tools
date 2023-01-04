// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package span

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/gopls/internal/lsp/safetoken"
	"golang.org/x/tools/internal/bug"
)

// Range represents a source code range in token.Pos form.
// It also carries the token.File that produced the positions, so that it is
// self contained.
//
// TODO(adonovan): move to safetoken.Range (but the Range.Span function must stay behind).
type Range struct {
	TokFile    *token.File // non-nil
	Start, End token.Pos   // both IsValid()
}

// NewRange creates a new Range from a token.File and two positions within it.
// The given start position must be valid; if end is invalid, start is used as
// the end position.
//
// (If you only have a token.FileSet, use file = fset.File(start). But
// most callers know exactly which token.File they're dealing with and
// should pass it explicitly. Not only does this save a lookup, but it
// brings us a step closer to eliminating the global FileSet.)
func NewRange(file *token.File, start, end token.Pos) Range {
	if file == nil {
		panic("nil *token.File")
	}
	if !start.IsValid() {
		panic("invalid start token.Pos")
	}
	if !end.IsValid() {
		end = start
	}

	// TODO(adonovan): ideally we would make this stronger assertion:
	//
	//   // Assert that file is non-nil and contains start and end.
	//   _ = file.Offset(start)
	//   _ = file.Offset(end)
	//
	// but some callers (e.g. packageCompletionSurrounding,
	// posToMappedRange) don't ensure this precondition.

	return Range{
		TokFile: file,
		Start:   start,
		End:     end,
	}
}

// IsPoint returns true if the range represents a single point.
func (r Range) IsPoint() bool {
	return r.Start == r.End
}

// Span converts a Range to a Span that represents the Range.
// It will fill in all the members of the Span, calculating the line and column
// information.
func (r Range) Span() (Span, error) {
	return FileSpan(r.TokFile, r.Start, r.End)
}

// FileSpan returns a span within the file referenced by start and
// end, using a token.File to translate between offsets and positions.
func FileSpan(file *token.File, start, end token.Pos) (Span, error) {
	if !start.IsValid() {
		return Span{}, fmt.Errorf("start pos is not valid")
	}
	var s Span
	var err error
	var startFilename string
	startFilename, s.v.Start.Line, s.v.Start.Column, err = position(file, start)
	if err != nil {
		return Span{}, err
	}
	s.v.URI = URIFromPath(startFilename)
	if end.IsValid() {
		var endFilename string
		endFilename, s.v.End.Line, s.v.End.Column, err = position(file, end)
		if err != nil {
			return Span{}, err
		}
		if endFilename != startFilename {
			return Span{}, fmt.Errorf("span begins in file %q but ends in %q", startFilename, endFilename)
		}
	}
	s.v.Start.clean()
	s.v.End.clean()
	s.v.clean()
	return s.WithOffset(file)
}

func position(tf *token.File, pos token.Pos) (string, int, int, error) {
	off, err := offset(tf, pos)
	if err != nil {
		return "", 0, 0, err
	}
	return positionFromOffset(tf, off)
}

func positionFromOffset(tf *token.File, offset int) (string, int, int, error) {
	if offset > tf.Size() {
		return "", 0, 0, fmt.Errorf("offset %d is beyond EOF (%d) in file %s", offset, tf.Size(), tf.Name())
	}
	pos := tf.Pos(offset)
	p := safetoken.Position(tf, pos)
	// TODO(golang/go#41029): Consider returning line, column instead of line+1, 1 if
	// the file's last character is not a newline.
	if offset == tf.Size() {
		return p.Filename, p.Line + 1, 1, nil
	}
	return p.Filename, p.Line, p.Column, nil
}

// offset is a copy of the Offset function in go/token, but with the adjustment
// that it does not panic on invalid positions.
func offset(tf *token.File, pos token.Pos) (int, error) {
	if int(pos) < tf.Base() || int(pos) > tf.Base()+tf.Size() {
		return 0, fmt.Errorf("invalid pos: %d not in [%d, %d]", pos, tf.Base(), tf.Base()+tf.Size())
	}
	return int(pos) - tf.Base(), nil
}

// Range converts a Span to a Range that represents the Span for the supplied
// File.
func (s Span) Range(tf *token.File) (Range, error) {
	s, err := s.WithOffset(tf)
	if err != nil {
		return Range{}, err
	}
	// go/token will panic if the offset is larger than the file's size,
	// so check here to avoid panicking.
	if s.Start().Offset() > tf.Size() {
		return Range{}, bug.Errorf("start offset %v is past the end of the file %v", s.Start(), tf.Size())
	}
	if s.End().Offset() > tf.Size() {
		return Range{}, bug.Errorf("end offset %v is past the end of the file %v", s.End(), tf.Size())
	}
	return Range{
		Start:   tf.Pos(s.Start().Offset()),
		End:     tf.Pos(s.End().Offset()),
		TokFile: tf,
	}, nil
}

// OffsetToLineCol8 converts a byte offset in the file corresponding to tf into
// 1-based line and utf-8 column indexes.
//
// TODO(adonovan): move to safetoken package for consistency?
func OffsetToLineCol8(tf *token.File, offset int) (int, int, error) {
	_, line, col8, err := positionFromOffset(tf, offset)
	return line, col8, err
}

// ToOffset converts a 1-based line and utf-8 column index into a byte offset
// in the file corresponding to tf.
func ToOffset(tf *token.File, line, col int) (int, error) {
	if line < 1 { // token.File.LineStart panics if line < 1
		return -1, fmt.Errorf("invalid line: %d", line)
	}

	lineMax := tf.LineCount() + 1
	if line > lineMax {
		return -1, fmt.Errorf("line %d is beyond end of file %v", line, lineMax)
	} else if line == lineMax {
		if col > 1 {
			return -1, fmt.Errorf("column is beyond end of file")
		}
		// at the end of the file, allowing for a trailing eol
		return tf.Size(), nil
	}
	pos := tf.LineStart(line)
	if !pos.IsValid() {
		// bug.Errorf here because LineStart panics on out-of-bound input, and so
		// should never return invalid positions.
		return -1, bug.Errorf("line is not in file")
	}
	// we assume that column is in bytes here, and that the first byte of a
	// line is at column 1
	pos += token.Pos(col - 1)

	// Debugging support for https://github.com/golang/go/issues/54655.
	if pos > token.Pos(tf.Base()+tf.Size()) {
		return 0, fmt.Errorf("ToOffset: column %d is beyond end of file", col)
	}

	return offset(tf, pos)
}
