// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package asn1spec

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

type Position struct {
	Filename string
	Line     int
	Column   int
}

func (p Position) String() string {
	if p.Filename == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Column)
	}

	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

type Error struct {
	Pos Position
	Msg string
}

func (e *Error) Error() string {
	return e.Pos.String() + ": " + e.Msg
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber
	tokCString
	tokBString
	tokHString
	tokField
	tokAssign
	tokLBrace
	tokRBrace
	tokLParen
	tokRParen
	tokLBrack
	tokRBrack
	tokLVersion
	tokRVersion
	tokComma
	tokDot
	tokRange
	tokEllipsis
	tokColon
	tokSemicolon
	tokPipe
	tokAt
	tokMinus
	tokExcl
	tokCaret
	tokLess
)

var tokenNames = [...]string{
	tokEOF:       "EOF",
	tokIdent:     "identifier",
	tokNumber:    "number",
	tokCString:   "cstring",
	tokBString:   "bstring",
	tokHString:   "hstring",
	tokField:     "field reference",
	tokAssign:    "::=",
	tokLBrace:    "{",
	tokRBrace:    "}",
	tokLParen:    "(",
	tokRParen:    ")",
	tokLBrack:    "[",
	tokRBrack:    "]",
	tokLVersion:  "[[",
	tokRVersion:  "]]",
	tokComma:     ",",
	tokDot:       ".",
	tokRange:     "..",
	tokEllipsis:  "...",
	tokColon:     ":",
	tokSemicolon: ";",
	tokPipe:      "|",
	tokAt:        "@",
	tokMinus:     "-",
	tokExcl:      "!",
	tokCaret:     "^",
	tokLess:      "<",
}

func (k tokenKind) String() string {
	if k >= 0 && int(k) < len(tokenNames) {
		return tokenNames[k]
	}

	return fmt.Sprintf("token(%d)", int(k))
}

type token struct {
	kind tokenKind
	lit  string
	pos  Position
}

func (t token) String() string {
	switch t.kind {
	case tokIdent, tokNumber, tokField, tokCString, tokBString, tokHString:
		return fmt.Sprintf("%s %q", t.kind, t.lit)
	default:
		return fmt.Sprintf("%q", t.kind.String())
	}
}

type scanner struct {
	filename  string
	src       []byte
	ch        rune
	offset    int
	rdOffset  int
	line      int
	lineStart int
}

func scan(filename string, src []byte) ([]token, error) {
	s := &scanner{filename: filename, src: src, line: 1}
	s.next()

	var toks []token

	for {
		t, err := s.scan()
		if err != nil {
			return nil, err
		}

		toks = append(toks, t)
		if t.kind == tokEOF {
			return toks, nil
		}
	}
}

func (s *scanner) next() {
	if s.ch == '\n' {
		s.line++
		s.lineStart = s.offset + 1
	}

	if s.rdOffset >= len(s.src) {
		s.offset = len(s.src)
		s.ch = -1

		return
	}

	s.offset = s.rdOffset

	r, w := rune(s.src[s.rdOffset]), 1
	if r >= utf8.RuneSelf {
		r, w = utf8.DecodeRune(s.src[s.rdOffset:])
	}

	s.rdOffset += w
	s.ch = r
}

func (s *scanner) peek() rune {
	if s.rdOffset >= len(s.src) {
		return -1
	}

	r, _ := utf8.DecodeRune(s.src[s.rdOffset:])

	return r
}

func (s *scanner) pos() Position {
	return Position{Filename: s.filename, Line: s.line, Column: utf8.RuneCount(s.src[s.lineStart:s.offset]) + 1}
}

func (s *scanner) errorf(pos Position, format string, args ...any) error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

func (s *scanner) skipSpaceAndComments() error {
	for {
		switch {
		case s.ch >= 0 && unicode.IsSpace(s.ch):
			s.next()
		case s.ch == '-' && s.peek() == '-':
			s.next()
			s.next()

			for s.ch != '\n' && s.ch >= 0 {
				if s.ch == '-' && s.peek() == '-' {
					s.next()
					s.next()

					break
				}

				s.next()
			}
		case s.ch == '/' && s.peek() == '*':
			if err := s.skipBlockComment(); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (s *scanner) skipBlockComment() error {
	start := s.pos()
	depth := 0

	for {
		switch {
		case s.ch < 0:
			return s.errorf(start, "block comment not terminated")
		case s.ch == '/' && s.peek() == '*':
			s.next()
			s.next()

			depth++
		case s.ch == '*' && s.peek() == '/':
			s.next()
			s.next()

			depth--
			if depth == 0 {
				return nil
			}
		default:
			s.next()
		}
	}
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func (s *scanner) scanIdentifier() string {
	start := s.offset

	for {
		switch {
		case isLetter(s.ch) || isDigit(s.ch):
			s.next()
		case s.ch == '-' && (isLetter(s.peek()) || isDigit(s.peek())):
			s.next()
		default:
			return string(s.src[start:s.offset])
		}
	}
}

func (s *scanner) scan() (token, error) {
	if err := s.skipSpaceAndComments(); err != nil {
		return token{}, err
	}

	pos := s.pos()
	tok := func(k tokenKind, lit string) (token, error) {
		return token{kind: k, lit: lit, pos: pos}, nil
	}

	switch ch := s.ch; {
	case ch < 0:
		return tok(tokEOF, "")
	case isLetter(ch):
		return tok(tokIdent, s.scanIdentifier())
	case isDigit(ch):
		start := s.offset
		for isDigit(s.ch) {
			s.next()
		}

		return tok(tokNumber, string(s.src[start:s.offset]))
	case ch == '&':
		s.next()

		if !isLetter(s.ch) {
			return token{}, s.errorf(pos, "invalid field reference")
		}

		return tok(tokField, "&"+s.scanIdentifier())
	case ch == '"':
		return s.scanCString(pos)
	case ch == '\'':
		return s.scanBHString(pos)
	}

	ch := s.ch
	s.next()

	switch ch {
	case ':':
		if s.ch == ':' && s.peek() == '=' {
			s.next()
			s.next()

			return tok(tokAssign, "::=")
		}

		return tok(tokColon, ":")
	case '.':
		if s.ch == '.' {
			s.next()

			if s.ch == '.' {
				s.next()

				return tok(tokEllipsis, "...")
			}

			return tok(tokRange, "..")
		}

		return tok(tokDot, ".")
	case '[':
		if s.ch == '[' {
			s.next()

			return tok(tokLVersion, "[[")
		}

		return tok(tokLBrack, "[")
	case ']':
		if s.ch == ']' {
			s.next()

			return tok(tokRVersion, "]]")
		}

		return tok(tokRBrack, "]")
	case '{':
		return tok(tokLBrace, "{")
	case '}':
		return tok(tokRBrace, "}")
	case '(':
		return tok(tokLParen, "(")
	case ')':
		return tok(tokRParen, ")")
	case ',':
		return tok(tokComma, ",")
	case ';':
		return tok(tokSemicolon, ";")
	case '|':
		return tok(tokPipe, "|")
	case '@':
		return tok(tokAt, "@")
	case '-':
		return tok(tokMinus, "-")
	case '!':
		return tok(tokExcl, "!")
	case '^':
		return tok(tokCaret, "^")
	case '<':
		return tok(tokLess, "<")
	}

	return token{}, s.errorf(pos, "unexpected character %q", ch)
}

func (s *scanner) scanCString(pos Position) (token, error) {
	s.next()

	var buf []rune

	for {
		switch {
		case s.ch < 0:
			return token{}, s.errorf(pos, "string literal not terminated")
		case s.ch == '"' && s.peek() == '"':
			s.next()
			s.next()

			buf = append(buf, '"')
		case s.ch == '"':
			s.next()

			return token{kind: tokCString, lit: string(buf), pos: pos}, nil
		default:
			buf = append(buf, s.ch)
			s.next()
		}
	}
}

func (s *scanner) scanBHString(pos Position) (token, error) {
	s.next()

	var buf []rune

	for s.ch != '\'' {
		if s.ch < 0 {
			return token{}, s.errorf(pos, "bstring/hstring literal not terminated")
		}

		if !unicode.IsSpace(s.ch) {
			buf = append(buf, s.ch)
		}

		s.next()
	}

	s.next()

	switch s.ch {
	case 'B':
		s.next()

		return token{kind: tokBString, lit: string(buf), pos: pos}, nil
	case 'H':
		s.next()

		return token{kind: tokHString, lit: string(buf), pos: pos}, nil
	}

	return token{}, s.errorf(pos, "bstring/hstring literal without B or H suffix")
}
