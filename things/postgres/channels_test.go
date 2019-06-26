//
// Copyright (c) 2018
// Mainflux
//
// SPDX-License-Identifier: Apache-2.0
//

package postgres_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mainflux/mainflux/things"
	"github.com/mainflux/mainflux/things/postgres"
	"github.com/mainflux/mainflux/things/uuid"
	"github.com/stretchr/testify/assert"
)

func TestChannelSave(t *testing.T) {
	email := "channel-save@example.com"
	channelRepo := postgres.NewChannelRepository(db)

	id, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	channel := things.Channel{
		ID:    id,
		Owner: email,
	}

	cases := []struct {
		desc    string
		channel things.Channel
		err     error
	}{
		{
			desc:    "create valid channel",
			channel: channel,
			err:     nil,
		},
		{
			desc: "create channel with invalid ID",
			channel: things.Channel{
				ID:    "invalid",
				Owner: email,
			},
			err: things.ErrMalformedEntity,
		},
		{
			desc: "create channel with invalid name",
			channel: things.Channel{
				ID:    id,
				Owner: email,
				Name:  invalidName,
			},
			err: things.ErrMalformedEntity,
		},
	}

	for _, tc := range cases {
		_, err := channelRepo.Save(tc.channel)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestChannelUpdate(t *testing.T) {
	email := "channel-update@example.com"
	chanRepo := postgres.NewChannelRepository(db)

	cid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	c := things.Channel{
		ID:    cid,
		Owner: email,
	}

	id, _ := chanRepo.Save(c)
	c.ID = id

	nonexistentChanID, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	cases := []struct {
		desc    string
		channel things.Channel
		err     error
	}{
		{
			desc:    "update existing channel",
			channel: c,
			err:     nil,
		},
		{
			desc: "update non-existing channel with existing user",
			channel: things.Channel{
				ID:    nonexistentChanID,
				Owner: email,
			},
			err: things.ErrNotFound,
		},
		{
			desc: "update existing channel ID with non-existing user",
			channel: things.Channel{
				ID:    c.ID,
				Owner: wrongValue,
			},
			err: things.ErrNotFound,
		},
		{
			desc: "update non-existing channel with non-existing user",
			channel: things.Channel{
				ID:    nonexistentChanID,
				Owner: wrongValue,
			},
			err: things.ErrNotFound,
		},
	}

	for _, tc := range cases {
		err := chanRepo.Update(tc.channel)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestSingleChannelRetrieval(t *testing.T) {
	email := "channel-single-retrieval@example.com"
	chanRepo := postgres.NewChannelRepository(db)
	thingRepo := postgres.NewThingRepository(db)

	thid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	thkey, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	th := things.Thing{
		ID:    thid,
		Owner: email,
		Key:   thkey,
	}
	th.ID, _ = thingRepo.Save(th)

	chid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	c := things.Channel{
		ID:    chid,
		Owner: email,
	}

	c.ID, _ = chanRepo.Save(c)
	chanRepo.Connect(email, c.ID, th.ID)

	nonexistentChanID, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	cases := map[string]struct {
		owner string
		ID    string
		err   error
	}{
		"retrieve channel with existing user": {
			owner: c.Owner,
			ID:    c.ID,
			err:   nil,
		},
		"retrieve channel with existing user, non-existing channel": {
			owner: c.Owner,
			ID:    nonexistentChanID,
			err:   things.ErrNotFound,
		},
		"retrieve channel with non-existing owner": {
			owner: wrongValue,
			ID:    c.ID,
			err:   things.ErrNotFound,
		},
		"retrieve channel with malformed ID": {
			owner: c.Owner,
			ID:    wrongValue,
			err:   things.ErrNotFound,
		},
	}

	for desc, tc := range cases {
		_, err := chanRepo.RetrieveByID(tc.owner, tc.ID)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", desc, tc.err, err))
	}
}

func TestMultiChannelRetrieval(t *testing.T) {
	email := "channel-multi-retrieval@example.com"
	chanRepo := postgres.NewChannelRepository(db)
	channelName := "channel_name"

	n := uint64(10)
	for i := uint64(0); i < n; i++ {
		chid, err := uuid.New().ID()
		require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

		c := things.Channel{
			ID:    chid,
			Owner: email,
		}

		if i == 0 {
			c.Name = channelName
		}

		chanRepo.Save(c)
	}

	cases := map[string]struct {
		owner  string
		offset uint64
		limit  uint64
		name   string
		size   uint64
	}{
		"retrieve all channels with existing owner": {
			owner:  email,
			offset: 0,
			limit:  n,
			size:   n,
		},
		"retrieve subset of channels with existing owner": {
			owner:  email,
			offset: n / 2,
			limit:  n,
			size:   n / 2,
		},
		"retrieve channels with non-existing owner": {
			owner:  wrongValue,
			offset: n / 2,
			limit:  n,
			size:   0,
		},
		"retrieve all channels with existing name": {
			owner:  email,
			offset: 0,
			limit:  n,
			name:   channelName,
			size:   1,
		},
		"retrieve all channels with non-existing name": {
			owner:  email,
			offset: 0,
			limit:  n,
			name:   "wrong",
			size:   0,
		},
	}

	for desc, tc := range cases {
		page, err := chanRepo.RetrieveAll(tc.owner, tc.offset, tc.limit, tc.name)
		size := uint64(len(page.Channels))
		assert.Equal(t, tc.size, size, fmt.Sprintf("%s: expected %d got %d\n", desc, tc.size, size))
		assert.Nil(t, err, fmt.Sprintf("%s: expected no error got %d\n", desc, err))
	}
}

func TestMultiChannelRetrievalByThing(t *testing.T) {
	email := "channel-multi-retrieval-by-thing@example.com"
	idp := uuid.New()
	chanRepo := postgres.NewChannelRepository(db)
	thingRepo := postgres.NewThingRepository(db)

	thid, err := idp.ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	tid, err := thingRepo.Save(things.Thing{
		ID:    thid,
		Owner: email,
	})
	require.Nil(t, err, fmt.Sprintf("unexpected error: %s", err))
	n := uint64(10)
	for i := uint64(0); i < n; i++ {
		chid, err := uuid.New().ID()
		require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
		c := things.Channel{
			ID:    chid,
			Owner: email,
		}
		cid, err := chanRepo.Save(c)
		require.Nil(t, err, fmt.Sprintf("unexpected error: %s", err))
		err = chanRepo.Connect(email, cid, tid)
		require.Nil(t, err, fmt.Sprintf("unexpected error: %s", err))
	}

	nonexistentThingID, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	cases := map[string]struct {
		owner  string
		thing  string
		offset uint64
		limit  uint64
		size   uint64
		err    error
	}{
		"retrieve all channels by thing with existing owner": {
			owner:  email,
			thing:  tid,
			offset: 0,
			limit:  n,
			size:   n,
		},
		"retrieve subset of channels by thing with existing owner": {
			owner:  email,
			thing:  tid,
			offset: n / 2,
			limit:  n,
			size:   n / 2,
		},
		"retrieve channels by thing with non-existing owner": {
			owner:  wrongValue,
			thing:  tid,
			offset: n / 2,
			limit:  n,
			size:   0,
		},
		"retrieve channels by non-existent thing": {
			owner:  email,
			thing:  nonexistentThingID,
			offset: 0,
			limit:  n,
			size:   0,
		},
		"retrieve channels with malformed UUID": {
			owner:  email,
			thing:  wrongValue,
			offset: 0,
			limit:  n,
			size:   0,
			err:    things.ErrNotFound,
		},
	}

	for desc, tc := range cases {
		page, err := chanRepo.RetrieveByThing(tc.owner, tc.thing, tc.offset, tc.limit)
		size := uint64(len(page.Channels))
		assert.Equal(t, tc.size, size, fmt.Sprintf("%s: expected %d got %d\n", desc, tc.size, size))
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected no error got %d\n", desc, err))
	}
}

func TestChannelRemoval(t *testing.T) {
	email := "channel-removal@example.com"
	chanRepo := postgres.NewChannelRepository(db)

	chid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	chanID, _ := chanRepo.Save(things.Channel{
		ID:    chid,
		Owner: email,
	})

	// show that the removal works the same for both existing and non-existing
	// (removed) channel
	for i := 0; i < 2; i++ {
		err := chanRepo.Remove(email, chanID)
		require.Nil(t, err, fmt.Sprintf("#%d: failed to remove channel due to: %s", i, err))

		_, err = chanRepo.RetrieveByID(email, chanID)
		require.Equal(t, things.ErrNotFound, err, fmt.Sprintf("#%d: expected %s got %s", i, things.ErrNotFound, err))
	}
}

func TestConnect(t *testing.T) {
	email := "channel-connect@example.com"
	thingRepo := postgres.NewThingRepository(db)

	thid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	thkey, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	thing := things.Thing{
		ID:       thid,
		Owner:    email,
		Key:      thkey,
		Metadata: map[string]interface{}{},
	}
	thingID, _ := thingRepo.Save(thing)

	chanRepo := postgres.NewChannelRepository(db)

	chid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	chanID, _ := chanRepo.Save(things.Channel{
		ID:    chid,
		Owner: email,
	})

	nonexistentThingID, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	nonexistentChanID, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	cases := []struct {
		desc    string
		owner   string
		chanID  string
		thingID string
		err     error
	}{
		{
			desc:    "connect existing user, channel and thing",
			owner:   email,
			chanID:  chanID,
			thingID: thingID,
			err:     nil,
		},
		{
			desc:    "connect connected channel and thing",
			owner:   email,
			chanID:  chanID,
			thingID: thingID,
			err:     nil,
		},
		{
			desc:    "connect with non-existing user",
			owner:   wrongValue,
			chanID:  chanID,
			thingID: thingID,
			err:     things.ErrNotFound,
		},
		{
			desc:    "connect non-existing channel",
			owner:   email,
			chanID:  nonexistentChanID,
			thingID: thingID,
			err:     things.ErrNotFound,
		},
		{
			desc:    "connect non-existing thing",
			owner:   email,
			chanID:  chanID,
			thingID: nonexistentThingID,
			err:     things.ErrNotFound,
		},
	}

	for _, tc := range cases {
		err := chanRepo.Connect(tc.owner, tc.chanID, tc.thingID)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestDisconnect(t *testing.T) {
	email := "channel-disconnect@example.com"
	thingRepo := postgres.NewThingRepository(db)

	thid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	thkey, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	thing := things.Thing{
		ID:       thid,
		Owner:    email,
		Key:      thkey,
		Metadata: map[string]interface{}{},
	}
	thingID, _ := thingRepo.Save(thing)

	chanRepo := postgres.NewChannelRepository(db)
	chid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	chanID, _ := chanRepo.Save(things.Channel{
		ID:    chid,
		Owner: email,
	})
	chanRepo.Connect(email, chanID, thingID)

	nonexistentThingID, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	nonexistentChanID, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	cases := []struct {
		desc    string
		owner   string
		chanID  string
		thingID string
		err     error
	}{
		{
			desc:    "disconnect connected thing",
			owner:   email,
			chanID:  chanID,
			thingID: thingID,
			err:     nil,
		},
		{
			desc:    "disconnect non-connected thing",
			owner:   email,
			chanID:  chanID,
			thingID: thingID,
			err:     things.ErrNotFound,
		},
		{
			desc:    "disconnect non-existing user",
			owner:   wrongValue,
			chanID:  chanID,
			thingID: thingID,
			err:     things.ErrNotFound,
		},
		{
			desc:    "disconnect non-existing channel",
			owner:   email,
			chanID:  nonexistentChanID,
			thingID: thingID,
			err:     things.ErrNotFound,
		},
		{
			desc:    "disconnect non-existing thing",
			owner:   email,
			chanID:  chanID,
			thingID: nonexistentThingID,
			err:     things.ErrNotFound,
		},
	}

	for _, tc := range cases {
		err := chanRepo.Disconnect(tc.owner, tc.chanID, tc.thingID)
		assert.Equal(t, tc.err, err, fmt.Sprintf("%s: expected %s got %s\n", tc.desc, tc.err, err))
	}
}

func TestHasThing(t *testing.T) {
	email := "channel-access-check@example.com"
	thingRepo := postgres.NewThingRepository(db)

	thid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	thkey, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	thing := things.Thing{
		ID:    thid,
		Owner: email,
		Key:   thkey,
	}
	thingID, _ := thingRepo.Save(thing)

	chanRepo := postgres.NewChannelRepository(db)
	chid, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))
	chanID, _ := chanRepo.Save(things.Channel{
		ID:    chid,
		Owner: email,
	})
	chanRepo.Connect(email, chanID, thingID)

	nonexistentChanID, err := uuid.New().ID()
	require.Nil(t, err, fmt.Sprintf("got unexpected error: %s", err))

	cases := map[string]struct {
		chanID    string
		key       string
		hasAccess bool
	}{
		"access check for thing that has access": {
			chanID:    chanID,
			key:       thing.Key,
			hasAccess: true,
		},
		"access check for thing without access": {
			chanID:    chanID,
			key:       wrongValue,
			hasAccess: false,
		},
		"access check for non-existing channel": {
			chanID:    nonexistentChanID,
			key:       thing.Key,
			hasAccess: false,
		},
	}

	for desc, tc := range cases {
		_, err := chanRepo.HasThing(tc.chanID, tc.key)
		hasAccess := err == nil
		assert.Equal(t, tc.hasAccess, hasAccess, fmt.Sprintf("%s: expected %t got %t\n", desc, tc.hasAccess, hasAccess))
	}
}
