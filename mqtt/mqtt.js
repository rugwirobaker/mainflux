// Copyright (c) 2015-2019
// Mainflux
//
// SPDX-License-Identifier: Apache-2.0

'use strict';

var http = require('http'),
    redis = require('redis'),
    net = require('net'),
    protobuf = require('protocol-buffers'),
    websocket = require('websocket-stream'),
    grpc = require('grpc'),
    fs = require('fs'),
    bunyan = require('bunyan'),
    logging = require('aedes-logging');

// pass a proto file as a buffer/string or pass a parsed protobuf-schema object
var config = {
        log_level: process.env.MF_MQTT_ADAPTER_LOG_LEVEL || 'error',
        instance_id: process.env.MF_MQTT_INSTANCE_ID || '',
        event_stream: 'mainflux.mqtt',
        mqtt_port: Number(process.env.MF_MQTT_ADAPTER_PORT) || 1883,
        ws_port: Number(process.env.MF_MQTT_ADAPTER_WS_PORT) || 8880,
        nats_url: process.env.MF_NATS_URL || 'nats://localhost:4222',
        redis_port: Number(process.env.MF_MQTT_ADAPTER_REDIS_PORT) || 6379,
        redis_host: process.env.MF_MQTT_ADAPTER_REDIS_HOST || 'localhost',
        redis_pass: process.env.MF_MQTT_ADAPTER_REDIS_PASS || 'mqtt',
        redis_db: Number(process.env.MF_MQTT_ADAPTER_REDIS_DB) || 0,
        es_port: Number(process.env.MF_MQTT_ADAPTER_ES_PORT) || 6379,
        es_host: process.env.MF_MQTT_ADAPTER_ES_HOST || 'localhost',
        es_pass: process.env.MF_MQTT_ADAPTER_ES_PASS || 'mqtt',
        es_db: Number(process.env.MF_MQTT_ADAPTER_ES_DB) || 0,
        client_tls: (process.env.MF_MQTT_ADAPTER_CLIENT_TLS == 'true') || false,
    	ca_certs: process.env.MF_MQTT_ADAPTER_CA_CERTS || '',
        concurrency: Number(process.env.MF_MQTT_CONCURRENT_MESSAGES) || 100,
        auth_url: process.env.MF_THINGS_URL || 'localhost:8181',
        schema_dir: process.argv[2] || '.',
    },
    logger = bunyan.createLogger({name: 'mqtt', level: config.log_level}),
    message = protobuf(fs.readFileSync(config.schema_dir + '/message.proto')),
    thingsSchema = grpc.load(config.schema_dir + '/internal.proto').mainflux,
    nats = require('nats').connect(config.nats_url),
    aedesRedis = require('aedes-persistence-redis')({
        port: config.redis_port,
        host: config.redis_host,
        password: config.redis_pass,
        db: config.redis_db
    }),
    mqRedis = require('mqemitter-redis')({
        port: config.redis_port,
        host: config.redis_host,
        password: config.redis_pass,
        db: config.redis_db
    }),
    aedes = require('aedes')({
        mq: mqRedis,
        persistence: aedesRedis,
        concurrency: config.concurrency
    }),
    things = (function() {
        var certs;
        if (config.client_tls) {
            certs = grpc.credentials.createSsl(config.ca_certs);
        } else {
            certs = grpc.credentials.createInsecure();
        }
        return new thingsSchema.ThingsService(config.auth_url, certs);
    })(),
    esclient = redis.createClient({
        port: config.es_port, 
        host: config.es_host,
        password: config.es_pass,
        db: config.es_db
    }),
    servers = [
        startMqtt(),
        startWs()
    ];

logging({
    instance: aedes,
    servers: servers,
    pinoOptions: {level: 30}
});

logger.level(config.log_level);

esclient.on('error', function(err) {
    logger.warn('error on redis connection: %s', err.message);
});

// MQTT over WebSocket
function startWs() {
    var server = http.createServer();
    websocket.createServer({server: server}, aedes.handle);
    server.listen(config.ws_port);
    return server;
}

function startMqtt() {
    return net.createServer(aedes.handle).listen(config.mqtt_port);
}

nats.subscribe('channel.>', {'queue':'mqtts'}, function (msg) {
    var m = message.RawMessage.decode(Buffer.from(msg)),
        packet, subtopic;
    if (m && m.protocol !== 'mqtt') {
        subtopic = m.subtopic !== '' ? '/' + m.subtopic.replace(/\./g, '/') : '';

        packet = {
            cmd: 'publish',
            qos: 2,
            topic: 'channels/' + m.channel + '/messages' + subtopic,
            payload: m.payload,
            retain: false
        };

        aedes.publish(packet);
    }
});

function parseTopic(topic) {
    // Topics are in the form `channels/<channel_id>/messages`
    // Subtopic's are in the form `channels/<channel_id>/messages/<subtopic>`
    return /^channels\/(.+?)\/messages\/?.*$/.exec(topic);
}

aedes.authorizePublish = function (client, packet, publish) {
    var channel = parseTopic(packet.topic);
    if (!channel) {
        logger.warn('unknown topic');
        publish(4); // Bad username or password
        return;
    }
    var channelId = channel[1],
        accessReq = {
            token: client.password,
            chanID: channelId
        },
        // Parse unlimited subtopics
        baseLength = 3, // First 3 elements which represents the base part of topic.
        isEmpty = function(value) { 
            return value !== ''; 
        },
        elements = packet.topic.split('/').slice(baseLength).join('.').split('.').filter(isEmpty),
        baseTopic = 'channel.' + channelId;
    // Remove empty elements
    for (var i = 0; i < elements.length; i++) {
        if (elements[i].length > 1 && (elements[i].includes('*') || elements[i].includes('>'))) {
            logger.warn('invalid subtopic');
            publish(4);
            return;
        }
    }
    var channelTopic = elements.length ? baseTopic + '.' + elements.join('.') : baseTopic,
        onAuthorize = function (err, res) {
            var rawMsg;
            if (!err) {
                logger.info('authorized publish');

                rawMsg = message.RawMessage.encode({
                    publisher: client.thingId,
                    channel: channelId,
                    subtopic: elements.join('.'),
                    protocol: 'mqtt',
                    payload: packet.payload
                });
                nats.publish(channelTopic, rawMsg);

                publish(0);
            } else {
                logger.warn('unauthorized publish: %s', err.message);
                publish(4); // Bad username or password
            }
        };

    things.CanAccess(accessReq, onAuthorize);
};


aedes.authorizeSubscribe = function (client, packet, subscribe) {
    var channel = parseTopic(packet.topic);
    if (!channel) {
        logger.warn('unknown topic');
        subscribe(4, packet); // Bad username or password
        return;
    }
    var channelId = channel[1],
        accessReq = {
            token: client.password,
            chanID: channelId
        },
        onAuthorize = function (err, res) {
            if (!err) {
                logger.info('authorized subscribe');
                subscribe(null, packet);
            } else {
                logger.warn('unauthorized subscribe: %s', err.message);
                subscribe(4, packet); // Bad username or password
            }
        };

    things.canAccess(accessReq, onAuthorize);
};

aedes.authenticate = function (client, username, password, acknowledge) {
    var pass = (password || '').toString(),
        identity = {value: pass},
        onIdentify = function(err, res) {
            if (!err) {
                client.thingId = res.value.toString() || '';
                client.id = client.id || client.thingId;
                client.password = pass;
                acknowledge(null, true);
                publishConnEvent(client.thingId, 'connect');
            } else {
                logger.warn('failed to authenticate client with key %s', pass);
                acknowledge(err, false);
            }
        };

    things.identify(identity, onIdentify);
};

aedes.on('clientDisconnect', function (client) {
    logger.info('disconnect client %s', client.id);
    client.password = null;
    publishConnEvent(client.thingId, 'disconnect');
});

aedes.on('clientError', function (client, err) {
    logger.warn('client error: client: %s, error: %s', client.id, err.message);
});

aedes.on('connectionError', function (client, err) {
    logger.warn('client error: client: %s, error: %s', client.id, err.message);
});

aedes.on('error', function(err) {
    logger.warn('aedes error: %s', err.message);
});

function publishConnEvent(id, type) {
    var onPublish = function(err) {
        if (err) {
            logger.warn('event publish failed: %s', err);
        }
    };
    esclient.xadd(config.event_stream, '*',
        'thing_id', id,
        'timestamp', Math.round((new Date()).getTime() / 1000),
        'event_type', type,
        'instance', config.instance_id,
        onPublish);
}