// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
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
	prefix := "wire: "
	if e.Param != "" {
		prefix += e.Param + ": "
	}

	if e.Missing {
		return prefix + "missing mandatory IE"
	}

	return prefix + e.Reason
}

func (e *IEError) Unwrap() error {
	if e.Missing {
		return ErrMissingIE
	}

	return ErrIncorrectIE
}

func (e *IEError) Cause() Cause {
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

func Missing(param string) error {
	return &IEError{Param: param, Missing: true}
}

func Incorrect(param string, optional bool, format string, args ...any) error {
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

type enum interface {
	~string
	Known() bool
}

func CheckEnum[E enum](param string, v E, p Policy, optional bool) error {
	if v == "" {
		if optional {
			return nil
		}

		return Missing(param)
	}

	if p == Strict && !v.Known() {
		return Incorrect(param, optional, "unknown value %q", string(v))
	}

	return nil
}

func CheckEnums[E enum](param string, vs []E, p Policy) error {
	for i, v := range vs {
		if err := CheckEnum(param+"/"+strconv.Itoa(i), v, p, false); err != nil {
			return err
		}
	}

	return nil
}

func CheckAll[T Checker](param string, vs []T, p Policy) error {
	for i, v := range vs {
		if err := v.Check(p); err != nil {
			return NestIndex(param, i, err)
		}
	}

	return nil
}

func CheckOptional[T Checker](param string, v *T, p Policy) error {
	if v == nil {
		return nil
	}

	return Nest(param, (*v).Check(p))
}

func CheckRequired[T Checker](param string, v *T, p Policy) error {
	if v == nil {
		return Missing(param)
	}

	return Nest(param, (*v).Check(p))
}

func present(param string, ok bool) error {
	if !ok {
		return Missing(param)
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

		return Missing(param)
	}

	if !re.MatchString(v) {
		return Incorrect(param, optional, "%q does not match %s", v, re)
	}

	return nil
}

func checkRange(param string, v, lo, hi int, optional bool) error {
	if v < lo || v > hi {
		return Incorrect(param, optional, "%d out of range [%d, %d]", v, lo, hi)
	}

	return nil
}

func checkRequiredRange(param string, v *int, lo, hi int) error {
	if v == nil {
		return Missing(param)
	}

	return checkRange(param, *v, lo, hi, false)
}

func checkOptionalRange(param string, v *int, lo, hi int) error {
	if v == nil {
		return nil
	}

	return checkRange(param, *v, lo, hi, true)
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func decodeValid[T any, P interface {
	*T
	Checker
}](raw []json.RawMessage) []T {
	if raw == nil {
		return nil
	}

	out := make([]T, 0, len(raw))

	for _, r := range raw {
		var v T
		if json.Unmarshal(r, &v) == nil && P(&v).Check(Lenient) == nil {
			out = append(out, v)
		}
	}

	return out
}

func decodeValidMap[T any, P interface {
	*T
	Checker
}](raw map[string]json.RawMessage, validKey func(string) bool, fill func(string, *T)) map[string]T {
	if raw == nil {
		return nil
	}

	out := make(map[string]T, len(raw))

	for k, r := range raw {
		if !validKey(k) {
			continue
		}

		var v T
		if json.Unmarshal(r, &v) != nil {
			continue
		}

		if fill != nil {
			fill(k, &v)
		}

		if P(&v).Check(Lenient) == nil {
			out[k] = v
		}
	}

	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys
}
