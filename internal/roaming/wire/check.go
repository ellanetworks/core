// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

type Policy int

const (
	Lenient Policy = iota
	Strict
)

type Checker interface {
	Check(p Policy) error
}

var (
	ErrMalformed   = errors.New("wire: malformed JSON")
	ErrMissingIE   = errors.New("wire: missing mandatory IE")
	ErrIncorrectIE = errors.New("wire: incorrect IE")
)

type IEError struct {
	Param    string
	Missing  bool
	Optional bool
	Reason   string
}

func (e *IEError) Error() string {
	if e.Missing {
		return fmt.Sprintf("wire: %s: missing mandatory IE", e.Param)
	}

	return fmt.Sprintf("wire: %s: %s", e.Param, e.Reason)
}

func (e *IEError) Unwrap() error {
	if e.Missing {
		return ErrMissingIE
	}

	return ErrIncorrectIE
}

func (e *IEError) Cause() string {
	switch {
	case e.Missing:
		return CauseMandatoryIEMissing
	case e.Optional:
		return CauseOptionalIEIncorrect
	default:
		return CauseMandatoryIEIncorrect
	}
}

func Decode[T any, P interface {
	*T
	Checker
}](data []byte) (T, error) {
	var v T

	if err := json.Unmarshal(data, &v); err != nil {
		var ie *IEError
		if errors.As(err, &ie) {
			return v, err
		}

		return v, fmt.Errorf("%w: %w", ErrMalformed, err)
	}

	return v, P(&v).Check(Lenient)
}

func Encode(c Checker) ([]byte, error) {
	if err := c.Check(Strict); err != nil {
		return nil, err
	}

	return json.Marshal(c)
}

func missing(param string) error {
	return &IEError{Param: param, Missing: true}
}

func incorrect(param string, optional bool, format string, args ...any) error {
	return &IEError{Param: param, Optional: optional, Reason: fmt.Sprintf(format, args...)}
}

func Nest(prefix string, err error) error {
	var ie *IEError
	if !errors.As(err, &ie) {
		return err
	}

	out := *ie
	out.Param = prefix + ie.Param

	return &out
}

func NestIndex(prefix string, i int, err error) error {
	return Nest(prefix+"/"+strconv.Itoa(i), err)
}

func requireKeys(data []byte, keys ...string) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	for _, k := range keys {
		if v, ok := obj[k]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return missing("/" + k)
		}
	}

	return nil
}

type enum interface {
	~string
	Known() bool
}

func checkEnum[E enum](param string, v E, p Policy, optional bool) error {
	if v == "" {
		if optional {
			return nil
		}

		return missing(param)
	}

	if p == Strict && !v.Known() {
		return incorrect(param, optional, "unknown value %q", string(v))
	}

	return nil
}

func checkEnums[E enum](param string, vs []E, p Policy) error {
	for i, v := range vs {
		if err := checkEnum(param+"/"+strconv.Itoa(i), v, p, false); err != nil {
			return err
		}
	}

	return nil
}

func oneOf[E ~string](v E, values ...E) bool {
	return slices.Contains(values, v)
}

func checkPattern(param string, v string, re *regexp.Regexp, optional bool) error {
	if v == "" {
		if optional {
			return nil
		}

		return missing(param)
	}

	if !re.MatchString(v) {
		return incorrect(param, optional, "%q does not match %s", v, re)
	}

	return nil
}

func checkRange(param string, v, lo, hi int, optional bool) error {
	if v < lo || v > hi {
		return incorrect(param, optional, "%d out of range [%d, %d]", v, lo, hi)
	}

	return nil
}

func checkAll[T Checker](param string, vs []T, p Policy) error {
	for i, v := range vs {
		if err := v.Check(p); err != nil {
			return NestIndex(param, i, err)
		}
	}

	return nil
}

func checkOptional[T Checker](param string, v *T, p Policy) error {
	if v == nil {
		return nil
	}

	if err := (*v).Check(p); err != nil {
		return Nest(param, err)
	}

	return nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}
