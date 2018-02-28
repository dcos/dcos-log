// Copyright 2016 Mesosphere, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store

import "testing"

// Smoketest that the store works at a high level
func TestStore(t *testing.T) {
	s := New()
	var sv interface{}
	var ok bool

	// Smoketest Set(), Get(), and Objects()
	s.Set("foo", "bar")
	sv, ok = s.Get("foo")
	if sv != "bar" {
		t.Fatalf("Expected key 'foo' to have value 'bar'. Got: %s", sv)
	}
	if ok != true {
		t.Fatalf("Expected ok to be true (but it wasn't!)")
	}
	if l := len(s.Objects()); l != 1 {
		t.Fatalf("Expected objects in the store to be 1. Got: %d", l)
	}

	// Smoketest Delete()
	s.Delete("foo")
	if l := len(s.Objects()); l != 0 {
		t.Fatalf("Expected objects in the store to be 0. Got: %d", l)
	}

	// Smoketest Supplant() and Size()
	someMap := make(map[string]interface{})
	testCases := []struct {
		key string
		val string
	}{
		{"foo2", "fooval2"},
		{"bar2", "barval2"},
		{"baz2", "bazval2"},
	}

	for _, tc := range testCases {
		someMap[tc.key] = tc.val
	}

	s.Supplant(someMap)
	if l := s.Size(); l != 3 {
		t.Fatalf("Expected 3 objects in the store. Got: %d", l)
	}
}

// When deleting an object in the store, the object should be removed completely
func TestStore_Delete(t *testing.T) {
	var s storeImpl
	s.objects = map[string]object{}
	s.objects["foo"] = object{contents: "bar"}

	s.Delete("foo")

	if l := len(s.objects); l != 0 {
		t.Fatalf("Expected the number of store objects to be 0. Got: %d", l)
	}

	if oc, ok := s.objects["foo"]; ok != false {
		t.Fatalf("Expected 'foo' to not be found (but it was!). Got: %s", oc)
	}
}

// When getting an object in the store, the object's contents should be
// returned if it exists. If the requested object doesn't exist, should return nil.
func TestStore_Get(t *testing.T) {
	var s storeImpl
	s.objects = map[string]object{}
	s.objects["foo"] = object{contents: "bar"}

	var val interface{}
	var ok bool

	val, ok = s.Get("foo")
	if val != "bar" {
		t.Fatalf("Expected value returned to be 'bar'. Got: %s", val)
	}
	if ok != true {
		t.Fatalf("Expected second assignment 'ok' to be true (but it wasn't!)")
	}

	val, ok = s.Get("someNonExistentKey")
	if val != nil {
		t.Fatalf("Expected contents of a non-existent key to be nil. Got: %v", val)
	}
	if ok != false {
		t.Fatalf("Expected second assignment 'ok' to be false (but it wasn't!)")
	}
}

// When getting objects in the store by regular expression, the objects should
// be returned if they exist. Otherwise, return an empty map.
func TestStore_GetByRegex(t *testing.T) {
	testCases := []struct {
		key string
		val string
	}{
		{"foo", "bar"},
		{"foo.bar", "baz"},
		{"foo.bar.baz", "quux"},
		{"bar", "foo"},
	}
	var s storeImpl
	s.objects = map[string]object{}

	for _, tc := range testCases {
		s.objects[tc.key] = object{contents: tc.val}
	}

	result, err := s.GetByRegex("foo.*")

	if err != nil {
		t.Fatalf("Expected error to be nil (but it wasn't!)")
	}

	if l := len(result); l != 3 {
		t.Fatalf("Expected 3 objects to be returned, got %d", l)
	}

	if _, ok := result["bar"]; ok {
		t.Fatalf("Expected result to not contain key 'bar' (but it did!)")
	}
}

// When getting all objects in the store, all objects should be returned.
// Nothing more, nothing less.
func TestStore_Objects(t *testing.T) {
	testCases := []struct {
		key string
		val string
	}{
		{"foo", "fooval"},
		{"bar", "barval"},
		{"baz", "bazval"},
	}

	var s storeImpl
	s.objects = map[string]object{}

	for _, tc := range testCases {
		s.objects[tc.key] = object{contents: tc.val}
	}

	o := s.Objects()

	if l := len(o); l != 3 {
		t.Fatalf("Expected 3 items in the store. Got: %d", l)
	}

	for _, tc := range testCases {
		if oc := o[tc.key]; oc != tc.val {
			t.Fatalf("Expected key '%s' to contain value '%s'. Got: %s", tc.key, tc.val, oc)
		}
	}
}

// When purging the store, all objects should be deleted.
func TestStore_Purge(t *testing.T) {
	testCases := []struct {
		key string
		val string
	}{
		{"foo", "fooval"},
		{"bar", "barval"},
		{"baz", "bazval"},
	}

	var s storeImpl
	s.objects = map[string]object{}

	for _, tc := range testCases {
		s.Set(tc.key, tc.val)
	}

	s.Purge()

	if l := len(s.objects); l != 0 {
		t.Fatalf("Expected 0 objects in the store. Got: %d", l)
	}

	for _, tc := range testCases {
		if oc := s.objects[tc.key].contents; oc != nil {
			t.Fatalf("Expected to not find any objects in the store. Got: %s", oc)
		}
	}
}

// When setting the value of an object in the store, the value should be set
// and retrievable, and other objects in the store should be untouched.
func TestStore_Set(t *testing.T) {
	testCases := []struct {
		key string
		val string
	}{
		{"foo", "fooval"},
		{"bar", "barval"},
		{"baz", "bazval"},
	}

	var s storeImpl
	s.objects = map[string]object{}

	for _, tc := range testCases {
		s.Set(tc.key, tc.val)
	}

	if l := len(s.objects); l != 3 {
		t.Fatalf("Expected 3 objects in the store. Got: %d", l)
	}

	for _, tc := range testCases {
		if oc := s.objects[tc.key].contents; oc != tc.val {
			t.Fatalf("Expected key '%s' to contain value '%s'. Got: %s", tc.key, tc.val, oc)
		}
	}
}

// When getting the number of objects in the store, the correct size should be
// returned as a positive int. When adding or removing items, the new size
// should be returned.
func TestStore_Size(t *testing.T) {
	testCases := []struct {
		key string
		val string
	}{
		{"foo", "fooval"},
		{"bar", "barval"},
		{"baz", "bazval"},
	}

	var s storeImpl
	s.objects = map[string]object{}

	for _, tc := range testCases {
		s.Set(tc.key, tc.val)
	}

	if l := s.Size(); l != 3 {
		t.Fatalf("Expected 3 objects in the store. Got: %d", l)
	}
}

// When replacing all objects in the store with a new map of store objects,
// only the new objects should exist.
func TestStore_Supplant(t *testing.T) {
	initialTestCases := []struct {
		key string
		val string
	}{
		{"foo1", "fooval1"},
		{"bar1", "barval1"},
		{"baz1", "bazval1"},
	}
	finalTestCases := []struct {
		key string
		val string
	}{
		{"foo2", "fooval2"},
		{"bar2", "barval2"},
	}

	var s storeImpl
	s.objects = map[string]object{}

	for _, tc := range initialTestCases {
		s.Set(tc.key, tc.val)
	}

	newMap := make(map[string]interface{})
	for _, tc := range finalTestCases {
		newMap[tc.key] = tc.val
	}

	s.Supplant(newMap)

	if l := s.Size(); l != 2 {
		t.Fatalf("Expected the store to contain 2 objects. Got: %d", l)
	}

	for _, tc := range finalTestCases {
		if oc := s.objects[tc.key].contents; oc != tc.val {
			t.Fatalf("Expected key '%s' to contain value '%s'. Got: %s", tc.key, tc.val, oc)
		}
	}
}
