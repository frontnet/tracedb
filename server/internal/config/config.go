/*
 * Copyright 2020 Saffat Technologies, Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"encoding/json"

	"github.com/unit-io/unitdb/server/internal/pkg/log"
)

const (
	MaxMessageSize = 65536 // Maximum message size allowed from/to the peer.
)

// Config represents main configuration.
type Config struct {
	// Default HTTP(S) address:port to listen on for websocket. Either a
	// numeric or a canonical name, e.g. ":80" or ":https". Could include a host name, e.g.
	// "localhost:80".
	// Could be blank: if TLS is not configured, will use ":80", otherwise ":443".
	// Can be overridden from the command line, see option --listen.
	Listen string `json:"listen"`

	// Default HTTP(S) address:port to listen on for grpc. Either a
	// numeric or a canonical name, e.g. ":80" or ":https". Could include a host name, e.g.
	// "localhost:80".
	// Could be blank: if TLS is not configured, will use ":80", otherwise ":443".
	// Can be overridden from the command line, see option --listen.
	GrpcListen string `json:"grpc_listen"`

	// Default logging level is "InfoLevel" so to enable the debug log set the "LogLevel" to "DebugLevel".
	LoggingLevel string `json:"logging_level"`

	// MaxMessageSize     int             `json:"max_message_size"`
	// // Maximum number of topic subscribers.
	// MaxSubscriberCount int             `json:"max_subscriber_count"`

	EncryptionConfig json.RawMessage `json:"encryption_config"`

	// Configs for subsystems
	Cluster json.RawMessage `json:"cluster_config"`

	// Config for database store
	DBPath      string          `json:"db_path"`
	StoreConfig json.RawMessage `json:"store_config"`

	// Config to expose runtime stats
	VarzPath string `json:"varz_path"`
}

// EncryptionConfig represents the configuration for the encryption.
type EncryptionConfig struct {

	// chacha20poly1305 encryption key for client Ids and topic keys. 32 random bytes base64-encoded.
	Key string `json:"key,omitempty"`

	// Key identifier. it is useful when you use multiple keys.
	Identifier string `json:"identifier"`

	// slealed flag tells if key in the configuration is sealed.
	Sealed bool `json:"slealed"`

	// timestamp is helpful to determine the latest key in case of keyroll over.
	Timestamp uint32 `json:"timestamp,omitempty"`
}

func (c *Config) Encryption(encrConfig json.RawMessage) EncryptionConfig {
	var encr EncryptionConfig
	if err := json.Unmarshal(encrConfig, &encr); err != nil {
		log.Fatal("config.Encryption", "error in parsing encryption config", err)
	}

	return encr
}

// StoreConfig represents the configuration for the store.
type StoreConfig struct {
	// Reset resets message store on service restart
	Reset bool `json:"reset"`
}

func (c *Config) Store(storeConfig json.RawMessage) StoreConfig {
	var store StoreConfig
	if err := json.Unmarshal(storeConfig, &store); err != nil {
		log.Fatal("config.Encryption", "error in parsing encryption config", err)
	}

	return store
}
