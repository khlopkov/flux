package mock

import (
	"github.com/InfluxCommunity/flux"
	"github.com/InfluxCommunity/flux/codes"
	"github.com/InfluxCommunity/flux/execute"
	"github.com/InfluxCommunity/flux/internal/errors"
)

type Transport struct {
	ProcessMessageFn func(m execute.Message) error
}

func (t *Transport) ProcessMessage(m execute.Message) error {
	return t.ProcessMessageFn(m)
}

func (t *Transport) RetractTable(id execute.DatasetID, key flux.GroupKey) error {
	return errors.New(codes.Unimplemented)
}
func (t *Transport) Process(id execute.DatasetID, tbl flux.Table) error {
	return errors.New(codes.Unimplemented)
}
func (t *Transport) UpdateWatermark(id execute.DatasetID, ts execute.Time) error {
	return errors.New(codes.Unimplemented)
}
func (t *Transport) UpdateProcessingTime(id execute.DatasetID, ts execute.Time) error {
	return errors.New(codes.Unimplemented)
}
func (t *Transport) Finish(id execute.DatasetID, err error) {
}
