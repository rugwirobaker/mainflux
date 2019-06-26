//
// Copyright (c) 2018
// Mainflux
//
// SPDX-License-Identifier: Apache-2.0
//

package cassandra

import (
	"fmt"

	"github.com/gocql/gocql"
	"github.com/mainflux/mainflux"
	"github.com/mainflux/mainflux/readers"
)

var _ readers.MessageRepository = (*cassandraRepository)(nil)

type cassandraRepository struct {
	session *gocql.Session
}

// New instantiates Cassandra message repository.
func New(session *gocql.Session) readers.MessageRepository {
	return cassandraRepository{session: session}
}

func (cr cassandraRepository) ReadAll(chanID string, offset, limit uint64, query map[string]string) []mainflux.Message {
	cql, values := buildQuery(chanID, offset, limit, query)

	iter := cr.session.Query(cql, values...).Iter()
	scanner := iter.Scanner()

	// skip first OFFSET rows
	for i := uint64(0); i < offset; i++ {
		if !scanner.Next() {
			break
		}
	}

	var floatVal, valueSum *float64
	var strVal, dataVal *string
	var boolVal *bool

	page := []mainflux.Message{}
	for scanner.Next() {
		var msg mainflux.Message
		scanner.Scan(&msg.Channel, &msg.Subtopic, &msg.Publisher, &msg.Protocol,
			&msg.Name, &msg.Unit, &floatVal, &strVal, &boolVal,
			&dataVal, &valueSum, &msg.Time, &msg.UpdateTime, &msg.Link)

		switch {
		case floatVal != nil:
			msg.Value = &mainflux.Message_FloatValue{FloatValue: *floatVal}
		case strVal != nil:
			msg.Value = &mainflux.Message_StringValue{StringValue: *strVal}
		case boolVal != nil:
			msg.Value = &mainflux.Message_BoolValue{BoolValue: *boolVal}
		case dataVal != nil:
			msg.Value = &mainflux.Message_DataValue{DataValue: *dataVal}
		}

		if valueSum != nil {
			msg.ValueSum = &mainflux.SumValue{Value: *valueSum}
		}

		page = append(page, msg)
	}

	if err := iter.Close(); err != nil {
		return []mainflux.Message{}
	}

	return page
}

func buildQuery(chanID string, offset, limit uint64, query map[string]string) (string, []interface{}) {
	var condSql string
	var values []interface{}

	cql := `SELECT channel, subtopic, publisher, protocol, name, unit,
			value, string_value, bool_value, data_value, value_sum, time,
			update_time, link FROM messages WHERE channel = ? %s LIMIT ?
			ALLOW FILTERING`

	values = append(values, chanID)

	for name, value := range query {
		switch name {
		case
			"channel",
			"subtopic",
			"publisher",
			"name",
			"protocol":
			condSql = fmt.Sprintf(`%s AND %s = ?`, condSql, name)
			values = append(values, value)
		}
	}

	values = append(values, offset+limit)
	return fmt.Sprintf(cql, condSql), values
}
