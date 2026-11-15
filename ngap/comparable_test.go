// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"reflect"
	"testing"
)

func requireComparable[T comparable]() reflect.Type { return reflect.TypeFor[T]() }

func identityTypesWithValueEquality() []reflect.Type {
	return []reflect.Type{
		requireComparable[PLMNIdentity](),
		requireComparable[TAI](),
		requireComparable[GUAMI](),
		requireComparable[NRCGI](),
		requireComparable[EUTRACGI](),
		requireComparable[AllocationAndRetentionPriority](),
		requireComparable[GTPTEID](),
		requireComparable[TransportLayerAddress](),
		requireComparable[GTPTunnel](),
		requireComparable[EndpointIPAddressAndPort](),
	}
}

func identityTypesWithPointerComponents() []reflect.Type {
	return []reflect.Type{
		requireComparable[SNSSAI](),
		requireComparable[UPTransportLayerInformation](),
		requireComparable[NGRANCGI](),
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
