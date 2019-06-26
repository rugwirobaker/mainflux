//
// Copyright (c) 2018
// Mainflux
//
// SPDX-License-Identifier: Apache-2.0
//

package things_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/mainflux/mainflux/things"
	"github.com/mainflux/mainflux/things/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	wrongID    = ""
	wrongValue = "wrong-value"
	email      = "user@example.com"
	token      = "token"
)

var (
	thing   = things.Thing{Name: "test"}
	channel = things.Channel{Name: "test"}
)

func newService(tokens map[string]string) things.Service {
	users := mocks.NewUsersService(tokens)
	conns := make(chan mocks.Connection)
	thingsRepo := mocks.NewThingRepository(conns)
	channelsRepo := mocks.NewChannelRepository(thingsRepo, conns)
	chanCache := mocks.NewChannelCache()
	thingCache := mocks.NewThingCache()
	idp := mocks.NewIdentityProvider()

	return things.New(users, thingsRepo, channelsRepo, chanCache, thingCache, idp)
}

func TestAddThing(t *testing.T) {
	svc := newService(map[string]string{token: email})

	cases := []struct {
		desc  string
		thing things.Thing
		key   string
		err   error
	}{
		{
			desc:  "add new thing",
			thing: things.Thing{Name: "a"},
			key:   token,
			err:   nil,
		},
		{
			desc:  "add thing with wrong credentials",
			thing: things.Thing{Name: "d"},
			key:   wrongValue,
			err:   things.ErrUnauthorizedAccess,
		},
	}

	for _, tc := range cases {
		_, err := svc.AddThing(tc.key, tc.thing)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestUpdateThing(t *testing.T) {
	svc := newService(map[string]string{token: email})
	saved, _ := svc.AddThing(token, thing)
	other := things.Thing{ID: wrongID, Key: "x"}

	cases := []struct {
		desc  string
		thing things.Thing
		key   string
		err   error
	}{
		{
			desc:  "update existing thing",
			thing: saved,
			key:   token,
			err:   nil,
		},
		{
			desc:  "update thing with wrong credentials",
			thing: saved,
			key:   wrongValue,
			err:   things.ErrUnauthorizedAccess,
		},
		{
			desc:  "update non-existing thing",
			thing: other,
			key:   token,
			err:   things.ErrNotFound,
		},
	}

	for _, tc := range cases {
		err := svc.UpdateThing(tc.key, tc.thing)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestViewThing(t *testing.T) {
	svc := newService(map[string]string{token: email})
	saved, _ := svc.AddThing(token, thing)

	cases := map[string]struct {
		id  string
		key string
		err error
	}{
		"view existing thing": {
			id:  saved.ID,
			key: token,
			err: nil,
		},
		"view thing with wrong credentials": {
			id:  saved.ID,
			key: wrongValue,
			err: things.ErrUnauthorizedAccess,
		},
		"view non-existing thing": {
			id:  wrongID,
			key: token,
			err: things.ErrNotFound,
		},
	}

	for desc, tc := range cases {
		_, err := svc.ViewThing(tc.key, tc.id)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}

func TestListThings(t *testing.T) {
	svc := newService(map[string]string{token: email})

	n := uint64(10)
	for i := uint64(0); i < n; i++ {
		svc.AddThing(token, thing)
	}

	cases := map[string]struct {
		key    string
		offset uint64
		limit  uint64
		size   uint64
		err    error
	}{
		"list all things": {
			key:    token,
			offset: 0,
			limit:  n,
			size:   n,
			err:    nil,
		},
		"list half": {
			key:    token,
			offset: n / 2,
			limit:  n,
			size:   n / 2,
			err:    nil,
		},
		"list last thing": {
			key:    token,
			offset: n - 1,
			limit:  n,
			size:   1,
			err:    nil,
		},
		"list empty set": {
			key:    token,
			offset: n + 1,
			limit:  n,
			size:   0,
			err:    nil,
		},
		"list with zero limit": {
			key:    token,
			offset: 1,
			limit:  0,
			size:   0,
			err:    nil,
		},
		"list with wrong credentials": {
			key:    wrongValue,
			offset: 0,
			limit:  0,
			size:   0,
			err:    things.ErrUnauthorizedAccess,
		},
	}

	for desc, tc := range cases {
		page, err := svc.ListThings(tc.key, tc.offset, tc.limit)
		size := uint64(len(page.Things))
		assert.Equal(t, tc.size, size, fmt.Sprintf("%s: expected %d got %d\n", desc, tc.size, size))
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}

func TestListThingsByChannel(t *testing.T) {
	svc := newService(map[string]string{token: email})

	sch, err := svc.CreateChannel(token, channel)
	require.Nil(t, err, fmt.Sprintf("unexpected error: %s", err))
	n := uint64(10)
	for i := uint64(0); i < n; i++ {
		sth, err := svc.AddThing(token, thing)
		require.Nil(t, err, fmt.Sprintf("unexpected error: %s", err))
		svc.Connect(token, sch.ID, sth.ID)
	}

	// Wait for things and channels to connect
	time.Sleep(time.Second)

	cases := map[string]struct {
		key     string
		channel string
		offset  uint64
		limit   uint64
		size    uint64
		err     error
	}{
		"list all things by existing channel": {
			key:     token,
			channel: sch.ID,
			offset:  0,
			limit:   n,
			size:    n,
			err:     nil,
		},
		"list half of things by existing channel": {
			key:     token,
			channel: sch.ID,
			offset:  n / 2,
			limit:   n,
			size:    n / 2,
			err:     nil,
		},
		"list last thing by existing channel": {
			key:     token,
			channel: sch.ID,
			offset:  n - 1,
			limit:   n,
			size:    1,
			err:     nil,
		},
		"list empty set of things by existing channel": {
			key:     token,
			channel: sch.ID,
			offset:  n + 1,
			limit:   n,
			size:    0,
			err:     nil,
		},
		"list things by existing channel with zero limit": {
			key:     token,
			channel: sch.ID,
			offset:  1,
			limit:   0,
			size:    0,
			err:     nil,
		},
		"list things by existing channel with wrong credentials": {
			key:     wrongValue,
			channel: sch.ID,
			offset:  0,
			limit:   0,
			size:    0,
			err:     things.ErrUnauthorizedAccess,
		},
		"list things by non-existent channel with wrong credentials": {
			key:     token,
			channel: "non-existent",
			offset:  0,
			limit:   10,
			size:    0,
			err:     nil,
		},
	}

	for desc, tc := range cases {
		page, err := svc.ListThingsByChannel(tc.key, tc.channel, tc.offset, tc.limit)
		size := uint64(len(page.Things))
		assert.Equal(t, tc.size, size, fmt.Sprintf("%s: expected %d got %d\n", desc, tc.size, size))
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}

func TestRemoveThing(t *testing.T) {
	svc := newService(map[string]string{token: email})
	saved, _ := svc.AddThing(token, thing)

	cases := []struct {
		desc string
		id   string
		key  string
		err  error
	}{
		{
			desc: "remove thing with wrong credentials",
			id:   saved.ID,
			key:  wrongValue,
			err:  things.ErrUnauthorizedAccess,
		},
		{
			desc: "remove existing thing",
			id:   saved.ID,
			key:  token,
			err:  nil,
		},
		{
			desc: "remove removed thing",
			id:   saved.ID,
			key:  token,
			err:  nil,
		},
		{
			desc: "remove non-existing thing",
			id:   wrongID,
			key:  token,
			err:  nil,
		},
	}

	for _, tc := range cases {
		err := svc.RemoveThing(tc.key, tc.id)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestCreateChannel(t *testing.T) {
	svc := newService(map[string]string{token: email})

	cases := []struct {
		desc    string
		channel things.Channel
		key     string
		err     error
	}{
		{
			desc:    "create channel",
			channel: channel,
			key:     token,
			err:     nil,
		},
		{
			desc:    "create channel with wrong credentials",
			channel: channel,
			key:     wrongValue,
			err:     things.ErrUnauthorizedAccess,
		},
	}

	for _, tc := range cases {
		_, err := svc.CreateChannel(tc.key, tc.channel)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestUpdateChannel(t *testing.T) {
	svc := newService(map[string]string{token: email})
	saved, _ := svc.CreateChannel(token, channel)
	other := things.Channel{ID: wrongID}

	cases := []struct {
		desc    string
		channel things.Channel
		key     string
		err     error
	}{
		{
			desc:    "update existing channel",
			channel: saved,
			key:     token,
			err:     nil,
		},
		{
			desc:    "update channel with wrong credentials",
			channel: saved,
			key:     wrongValue,
			err:     things.ErrUnauthorizedAccess,
		},
		{
			desc:    "update non-existing channel",
			channel: other,
			key:     token,
			err:     things.ErrNotFound,
		},
	}

	for _, tc := range cases {
		err := svc.UpdateChannel(tc.key, tc.channel)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestViewChannel(t *testing.T) {
	svc := newService(map[string]string{token: email})
	saved, _ := svc.CreateChannel(token, channel)

	cases := map[string]struct {
		id  string
		key string
		err error
	}{
		"view existing channel": {
			id:  saved.ID,
			key: token,
			err: nil,
		},
		"view channel with wrong credentials": {
			id:  saved.ID,
			key: wrongValue,
			err: things.ErrUnauthorizedAccess,
		},
		"view non-existing channel": {
			id:  wrongID,
			key: token,
			err: things.ErrNotFound,
		},
	}

	for desc, tc := range cases {
		_, err := svc.ViewChannel(tc.key, tc.id)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}

func TestListChannels(t *testing.T) {
	svc := newService(map[string]string{token: email})

	n := uint64(10)
	for i := uint64(0); i < n; i++ {
		svc.CreateChannel(token, channel)
	}
	cases := map[string]struct {
		key    string
		offset uint64
		limit  uint64
		size   uint64
		err    error
	}{
		"list all channels": {
			key:    token,
			offset: 0,
			limit:  n,
			size:   n,
			err:    nil,
		},
		"list half": {
			key:    token,
			offset: n / 2,
			limit:  n,
			size:   n / 2,
			err:    nil,
		},
		"list last channel": {
			key:    token,
			offset: n - 1,
			limit:  n,
			size:   1,
			err:    nil,
		},
		"list empty set": {
			key:    token,
			offset: n + 1,
			limit:  n,
			size:   0,
			err:    nil,
		},
		"list with zero limit": {
			key:    token,
			offset: 1,
			limit:  0,
			size:   0,
			err:    nil,
		},
		"list with wrong credentials": {
			key:    wrongValue,
			offset: 0,
			limit:  0,
			size:   0,
			err:    things.ErrUnauthorizedAccess,
		},
	}

	for desc, tc := range cases {
		page, err := svc.ListChannels(tc.key, tc.offset, tc.limit)
		size := uint64(len(page.Channels))
		assert.Equal(t, tc.size, size, fmt.Sprintf("%s: expected %d got %d\n", desc, tc.size, size))
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}

func TestListChannelsByThing(t *testing.T) {
	svc := newService(map[string]string{token: email})

	sth, err := svc.AddThing(token, thing)
	require.Nil(t, err, fmt.Sprintf("unexpected error: %s", err))
	n := uint64(10)
	for i := uint64(0); i < n; i++ {
		sch, err := svc.CreateChannel(token, channel)
		require.Nil(t, err, fmt.Sprintf("unexpected error: %s", err))
		svc.Connect(token, sch.ID, sth.ID)
	}

	// Wait for things and channels to connect.
	time.Sleep(time.Second)

	cases := map[string]struct {
		key    string
		thing  string
		offset uint64
		limit  uint64
		size   uint64
		err    error
	}{
		"list all channels by existing thing": {
			key:    token,
			thing:  sth.ID,
			offset: 0,
			limit:  n,
			size:   n,
			err:    nil,
		},
		"list half of channels by existing thing": {
			key:    token,
			thing:  sth.ID,
			offset: n / 2,
			limit:  n,
			size:   n / 2,
			err:    nil,
		},
		"list last channel by existing thing": {
			key:    token,
			thing:  sth.ID,
			offset: n - 1,
			limit:  n,
			size:   1,
			err:    nil,
		},
		"list empty set of channels by existing thing": {
			key:    token,
			thing:  sth.ID,
			offset: n + 1,
			limit:  n,
			size:   0,
			err:    nil,
		},
		"list channels by existing thing with zero limit": {
			key:    token,
			thing:  sth.ID,
			offset: 1,
			limit:  0,
			size:   0,
			err:    nil,
		},
		"list channels by existing thing with wrong credentials": {
			key:    wrongValue,
			thing:  sth.ID,
			offset: 0,
			limit:  0,
			size:   0,
			err:    things.ErrUnauthorizedAccess,
		},
		"list channels by non-existent thing": {
			key:    token,
			thing:  "non-existent",
			offset: 0,
			limit:  10,
			size:   0,
			err:    nil,
		},
	}

	for desc, tc := range cases {
		page, err := svc.ListChannelsByThing(tc.key, tc.thing, tc.offset, tc.limit)
		size := uint64(len(page.Channels))
		assert.Equal(t, tc.size, size, fmt.Sprintf("%s: expected %d got %d\n", desc, tc.size, size))
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}

func TestRemoveChannel(t *testing.T) {
	svc := newService(map[string]string{token: email})
	saved, _ := svc.CreateChannel(token, channel)

	cases := []struct {
		desc string
		id   string
		key  string
		err  error
	}{
		{
			desc: "remove channel with wrong credentials",
			id:   saved.ID,
			key:  wrongValue,
			err:  things.ErrUnauthorizedAccess,
		},
		{
			desc: "remove existing channel",
			id:   saved.ID,
			key:  token,
			err:  nil,
		},
		{
			desc: "remove removed channel",
			id:   saved.ID,
			key:  token,
			err:  nil,
		},
		{
			desc: "remove non-existing channel",
			id:   saved.ID,
			key:  token,
			err:  nil,
		},
	}

	for _, tc := range cases {
		err := svc.RemoveChannel(tc.key, tc.id)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestConnect(t *testing.T) {
	svc := newService(map[string]string{token: email})

	sth, _ := svc.AddThing(token, thing)
	sch, _ := svc.CreateChannel(token, channel)

	cases := []struct {
		desc    string
		key     string
		chanID  string
		thingID string
		err     error
	}{
		{
			desc:    "connect thing",
			key:     token,
			chanID:  sch.ID,
			thingID: sth.ID,
			err:     nil,
		},
		{
			desc:    "connect thing with wrong credentials",
			key:     wrongValue,
			chanID:  sch.ID,
			thingID: sth.ID,
			err:     things.ErrUnauthorizedAccess,
		},
		{
			desc:    "connect thing to non-existing channel",
			key:     token,
			chanID:  wrongID,
			thingID: sth.ID,
			err:     things.ErrNotFound,
		},
		{
			desc:    "connect non-existing thing to channel",
			key:     token,
			chanID:  sch.ID,
			thingID: wrongID,
			err:     things.ErrNotFound,
		},
	}

	for _, tc := range cases {
		err := svc.Connect(tc.key, tc.chanID, tc.thingID)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestDisconnect(t *testing.T) {
	svc := newService(map[string]string{token: email})

	sth, _ := svc.AddThing(token, thing)
	sch, _ := svc.CreateChannel(token, channel)
	svc.Connect(token, sch.ID, sth.ID)

	cases := []struct {
		desc    string
		key     string
		chanID  string
		thingID string
		err     error
	}{
		{
			desc:    "disconnect connected thing",
			key:     token,
			chanID:  sch.ID,
			thingID: sth.ID,
			err:     nil,
		},
		{
			desc:    "disconnect disconnected thing",
			key:     token,
			chanID:  sch.ID,
			thingID: sth.ID,
			err:     things.ErrNotFound,
		},
		{
			desc:    "disconnect with wrong credentials",
			key:     wrongValue,
			chanID:  sch.ID,
			thingID: sth.ID,
			err:     things.ErrUnauthorizedAccess,
		},
		{
			desc:    "disconnect from non-existing channel",
			key:     token,
			chanID:  wrongID,
			thingID: sth.ID,
			err:     things.ErrNotFound,
		},
		{
			desc:    "disconnect non-existing thing",
			key:     token,
			chanID:  sch.ID,
			thingID: wrongID,
			err:     things.ErrNotFound,
		},
	}

	for _, tc := range cases {
		err := svc.Disconnect(tc.key, tc.chanID, tc.thingID)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}

}

func TestCanAccess(t *testing.T) {
	svc := newService(map[string]string{token: email})

	sth, _ := svc.AddThing(token, thing)
	sch, _ := svc.CreateChannel(token, channel)
	svc.Connect(token, sch.ID, sth.ID)

	cases := map[string]struct {
		key     string
		channel string
		err     error
	}{
		"allowed access": {
			key:     sth.Key,
			channel: sch.ID,
			err:     nil,
		},
		"not-connected cannot access": {
			key:     wrongValue,
			channel: sch.ID,
			err:     things.ErrUnauthorizedAccess,
		},
		"access to non-existing channel": {
			key:     sth.Key,
			channel: wrongID,
			err:     things.ErrUnauthorizedAccess,
		},
	}

	for desc, tc := range cases {
		_, err := svc.CanAccess(tc.channel, tc.key)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}

func TestIdentify(t *testing.T) {
	svc := newService(map[string]string{token: email})

	sth, _ := svc.AddThing(token, thing)

	cases := map[string]struct {
		key string
		id  string
		err error
	}{
		"identify existing thing": {
			key: sth.Key,
			id:  sth.ID,
			err: nil,
		},
		"identify non-existing thing": {
			key: wrongValue,
			id:  wrongID,
			err: things.ErrUnauthorizedAccess,
		},
	}

	for desc, tc := range cases {
		id, err := svc.Identify(tc.key)
		assert.Equal(t, tc.id, id, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.id, id))
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}
