// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"reflect"
	"testing"
)

func requireComparable[T comparable]() reflect.Type { return reflect.TypeFor[T]() }

func identityTypesWithValueEquality() []reflect.Type {
	return []reflect.Type{
		requireComparable[PLMNIdentity](),
		requireComparable[TAI](),
		requireComparable[GUMMEI](),
		requireComparable[EUTRANCGI](),
		requireComparable[AllocationAndRetentionPriority](),
		requireComparable[GTPTEID](),
		requireComparable[TransportLayerAddress](),
		requireComparable[TransportInformation](),
		requireComparable[ERABSetupItemCtxtSURes](),
		requireComparable[ERABToBeSwitchedULItem](),
		requireComparable[ERABToBeModifiedItemBearerModInd](),
		requireComparable[NRCGI](),
		requireComparable[UES1APIDPair](),
		requireComparable[SecurityAlgorithms](),
	}
}

func identityTypesWithPointerComponents() []reflect.Type {
	return []reflect.Type{
		requireComparable[ENBID](),
		requireComparable[GlobalENBID](),
		requireComparable[GNBIdentity](),
		requireComparable[GlobalRANNodeID](),
		requireComparable[UES1APIDs](),
		requireComparable[TunnelInformation](),
	}
}

func referenceFree(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return true
	}

	seen[t] = true

	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func, reflect.Interface, reflect.Chan, reflect.UnsafePointer:
		return false
	case reflect.Array:
		return referenceFree(t.Elem(), seen)
	case reflect.Struct:
		for i := range t.NumField() {
			if !referenceFree(t.Field(i).Type, seen) {
				return false
			}
		}
	}

	return true
}

func TestIdentityTypesHaveValueEquality(t *testing.T) {
	for _, typ := range identityTypesWithValueEquality() {
		if !referenceFree(typ, map[reflect.Type]bool{}) {
			t.Errorf("%v holds a pointer, slice, map or interface, so == does not compare its value", typ)
		}
	}

	for _, typ := range identityTypesWithPointerComponents() {
		if referenceFree(typ, map[reflect.Type]bool{}) {
			t.Errorf("%v now has value equality: move it to identityTypesWithValueEquality", typ)
		}
	}
}
