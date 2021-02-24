/*
Copyright 2017 The Nuclio Authors.

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

package file

import (
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/loggerus"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type factory struct{}

func (f *factory) Create(name string,
	loggerSinkConfiguration *platformconfig.LoggerSinkWithLevel) (logger.Logger, error) {

	configuration, err := NewConfiguration(name, loggerSinkConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create prometheus pull configuration")
	}

	return loggerus.CreateFileLogger(name,
		configuration.Level,
		configuration.FormatterKind,
		configuration.FilePath,
		!configuration.Color)
}

// register factory
func init() {
	loggersink.RegistrySingleton.Register("file", &factory{})
}
