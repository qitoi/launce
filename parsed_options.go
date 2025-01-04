/*
 *  Copyright 2024 qitoi
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package launce

import (
	"github.com/vmihailenco/msgpack/v5"
)

var (
	_ msgpack.CustomDecoder = (*ParsedOptions)(nil)
)

// ParsedOptions is the command line options of locust.
type ParsedOptions struct {
	// Common Options
	Locustfile   string    `msgpack:"locustfile"`    // --locustfile
	Config       string    `msgpack:"config"`        // --config
	Host         string    `msgpack:"host"`          // --host
	NumUsers     *int64    `msgpack:"num_users"`     // --users
	SpawnRate    *float64  `msgpack:"spawn_rate"`    // --spawn-rate
	HatchRate    int64     `msgpack:"hatch_rate"`    // --hatch-rate
	RunTime      *int64    `msgpack:"run_time"`      // --run-time
	ListCommands bool      `msgpack:"list_commands"` // --list
	ConfigUsers  *[]string `msgpack:"config_users"`  // --config-users

	// Web UI Options
	WebHost            string  `msgpack:"web_host"`              // --web-host
	WebHostDisplayName string  `msgpack:"web_host_display_name"` // --web-host-display-name
	WebPort            int64   `msgpack:"web_port"`              // --web-port
	Headless           bool    `msgpack:"headless"`              // --headless
	Autostart          bool    `msgpack:"autostart"`             // --autostart
	Autoquit           int64   `msgpack:"autoquit"`              // --autoquit
	Headful            bool    `msgpack:"headful"`               // --headful
	WebAuth            *string `msgpack:"web_auth"`              // --web-auth
	WebLogin           bool    `msgpack:"web_login"`             // --web-login
	TLSCert            string  `msgpack:"tls_cert"`              // --tls-key
	TLSKey             string  `msgpack:"tls_key"`               // --tls-cert
	ClassPicker        bool    `msgpack:"class_picker"`          // --class-picker
	BuildPath          string  `msgpack:"build_path"`            // --build-path
	LegacyUI           bool    `msgpack:"legacy_ui"`             // --legacy-ui
	WebBasePath        string  `msgpack:"web_base_path"`         // --web-base-path

	// Master Options
	Master               bool   `msgpack:"master"`                  // --master
	MasterBindHost       string `msgpack:"master_bind_host"`        // --master-bind-host
	MasterBindPort       int64  `msgpack:"master_bind_port"`        // --master-bind-port
	ExpectWorkers        int64  `msgpack:"expect_workers"`          // --expect-workers
	ExpectWorkersMaxWait int64  `msgpack:"expect_workers_max_wait"` // --expect-worker-max-wait
	EnableRebalancing    bool   `msgpack:"enable_rebalancing"`      // --enable-rebalancing
	ExpectSlaves         bool   `msgpack:"expect_slaves"`           // --expect-slaves

	// Worker Options
	Worker     bool   `msgpack:"worker"`      // --worker
	Processes  *int64 `msgpack:"processes"`   // --processes
	Slave      bool   `msgpack:"slave"`       // --slave
	MasterHost string `msgpack:"master_host"` // --master-host
	MasterPort int    `msgpack:"master_port"` // --master-port

	// Tag Options
	Tags        *[]string `msgpack:"tags"`         // --tags
	ExcludeTags *[]string `msgpack:"exclude_tags"` // --exclude-tags

	// Stats Options
	CsvPrefix           *string `msgpack:"csv_prefix"`            // --csv
	StatsHistoryEnabled bool    `msgpack:"stats_history_enabled"` // --csv-full-history
	PrintStats          bool    `msgpack:"print_stats"`           // --print-stats
	OnlySummary         bool    `msgpack:"only_summary"`          // --only-summary
	ResetStats          bool    `msgpack:"reset_stats"`           // --reset-stats
	HtmlFile            *string `msgpack:"html_file"`             // --html
	Json                bool    `msgpack:"json"`                  // --json

	// Log Options
	SkipLogSetup bool    `msgpack:"skip_log_setup"` // --skip-log-setup
	Loglevel     string  `msgpack:"loglevel"`       // --loglevel
	Logfile      *string `msgpack:"logfile"`        // --logfile

	// Other Options
	ShowTaskRatio     bool  `msgpack:"show_task_ratio"`      // --show-task-ratio
	ShowTaskRatioJson bool  `msgpack:"show_task_ratio_json"` // --show-task-ratio-json
	ExitCodeOnError   int64 `msgpack:"exit_code_on_error"`   // --exit-code-on-error
	StopTimeout       int64 `msgpack:"stop_timeout"`         // --stop-timeout
	EqualWeights      bool  `msgpack:"equal_weights"`        // --equal-weights

	// User Classes Options
	UserClasses []string `msgpack:"user_classes"` // <UserClass1 UserClass2>

	raw msgpack.RawMessage `msgpack:"-"`
}

// DecodeMsgpack decodes the msgpack data.
func (p *ParsedOptions) DecodeMsgpack(dec *msgpack.Decoder) error {
	// avoid unmarshal infinite loop
	type parsedOptions ParsedOptions
	tp := (*parsedOptions)(p)

	raw, err := dec.DecodeRaw()
	if err != nil {
		return err
	}
	if err := msgpack.Unmarshal(raw, tp); err != nil {
		return err
	}

	p.raw = raw

	return nil
}

// Extract extracts the parsed options to the given value.
func (p *ParsedOptions) Extract(v interface{}) error {
	return msgpack.Unmarshal(p.raw, v)
}
