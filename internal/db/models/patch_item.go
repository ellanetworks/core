package models

import "github.com/omec-project/openapi/models"

type PatchOperation string

const (
	PatchOperation_ADD     PatchOperation = "add"
	PatchOperation_COPY    PatchOperation = "copy"
	PatchOperation_MOVE    PatchOperation = "move"
	PatchOperation_REMOVE  PatchOperation = "remove"
	PatchOperation_REPLACE PatchOperation = "replace"
	PatchOperation_TEST    PatchOperation = "test"
)

type PatchItem struct {
	Op    models.PatchOperation `json:"op" yaml:"op" bson:"op" mapstructure:"Op"`
	Path  string                `json:"path" yaml:"path" bson:"path" mapstructure:"Path"`
	From  string                `json:"from,omitempty" yaml:"from" bson:"from" mapstructure:"From"`
	Value interface{}           `json:"value,omitempty" yaml:"value" bson:"value" mapstructure:"Value"`
}
