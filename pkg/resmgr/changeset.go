/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resmgr

import "fmt"

// Action resents the action type performed by the reconciliation process.
type Action string

const (
	CreatedAction    Action = "created"
	ConfiguredAction Action = "configured"
	UnchangedAction  Action = "unchanged"
	DeletedAction    Action = "deleted"
)

// ChangeSet holds the result of the reconciliation of an object collection.
type ChangeSet struct {
	Entries []ChangeSetEntry
}

func NewChangeSet() *ChangeSet {
	return &ChangeSet{Entries: []ChangeSetEntry{}}
}

func (c *ChangeSet) Add(e ChangeSetEntry) {
	c.Entries = append(c.Entries, e)
}

func (c *ChangeSet) AddAll(e []ChangeSetEntry) {
	c.Entries = append(c.Entries, e...)
}

// ChangeSetEntry defines the result of an action performed on an object.
type ChangeSetEntry struct {
	// Subject represents the Object ID in the format 'kind/namespace/name'.
	Subject string
	// Action represents the action type taken by the reconciler for this object.
	Action string
	// Diff contains the object diff.
	Diff string
}

func (e ChangeSetEntry) String() string {
	return fmt.Sprintf("%s %s", e.Subject, e.Action)
}
