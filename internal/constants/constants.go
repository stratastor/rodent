/*
 * Copyright 2024-2025 Raamsri Kumar <raam@tinkershack.in> 
 * Copyright 2024-2025 The StrataSTOR Authors and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package constants

const (
	RodentVersion     = "v0.0.1"
	RodentPIDFilePath = "/var/run/rodent.pid"

	// config
	SystemConfigDir = "/etc/rodent"
	UserConfigDir   = "~/.rodent"
	ConfigFileName  = "rodent.yml"
	StateFileName   = "rodent_state.yml"
)
