/**
 *
 * (c) Copyright Ascensio System SIA 2023
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
 *
 */

package log

import (
	"os"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log/hook"
	"github.com/natefinch/lumberjack"
	elastic "github.com/olivere/elastic/v7"
	"github.com/sirupsen/logrus"
)

type LogLevel int

const (
	LEVEL_TRACE   LogLevel = 1
	LEVEL_DEBUG   LogLevel = 2
	LEVEL_INFO    LogLevel = 3
	LEVEL_WARNING LogLevel = 4
	LEVEL_ERROR   LogLevel = 5
	LEVEL_FATAL   LogLevel = 6
)

var levels = map[LogLevel]logrus.Level{
	LEVEL_TRACE:   logrus.TraceLevel,
	LEVEL_DEBUG:   logrus.DebugLevel,
	LEVEL_INFO:    logrus.InfoLevel,
	LEVEL_WARNING: logrus.WarnLevel,
	LEVEL_ERROR:   logrus.ErrorLevel,
	LEVEL_FATAL:   logrus.FatalLevel,
}

// LogrusLogger is a logrus logger wrapper.
type LogrusLogger struct {
	logger *logrus.Logger
	config config.LoggerConfig
}

// createElasticHook opens a new elastic client and generates an elastic hook.
func createElasticHook(config config.ElasticLogConfig) (*hook.ElasticHook, error) {
	client, err := elastic.NewClient(
		elastic.SetURL(config.Address),
		elastic.SetSniff(false),
		elastic.SetBasicAuth(config.BasicAuthUsername, config.BasicAuthPassword),
		elastic.SetHealthcheck(config.HealthcheckEnabled),
		elastic.SetGzip(config.GzipEnabled),
	)

	if err != nil {
		return nil, &LogElasticInitializationError{
			Address: config.Address,
			Cause:   err,
		}
	}

	if config.Bulk {
		return hook.NewBulkProcessorElasticHook(client, config.Address, levels[LogLevel(config.Level)], config.Index)
	}

	if config.Async {
		return hook.NewAsyncElasticHook(client, config.Address, levels[LogLevel(config.Level)], config.Index)
	}

	return hook.NewElasticHook(client, config.Address, levels[LogLevel(config.Level)], config.Index)
}

// NewLogrusLogger creates a new logger compliant with the Logger interface.
func NewLogrusLogger(config *config.LoggerConfig) (Logger, error) {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		DisableColors: !config.Logger.Color,
		FullTimestamp: true,
	})

	if lvl, ok := levels[LogLevel(config.Logger.Level)]; ok {
		log.SetLevel(lvl)
	}

	log.SetReportCaller(true)
	log.SetOutput(os.Stdout)

	if config.Logger.File.Filename != "" {
		log.SetOutput(&lumberjack.Logger{
			Filename:   config.Logger.File.Filename,
			MaxSize:    config.Logger.File.MaxSize,
			MaxBackups: config.Logger.File.MaxBackups,
			MaxAge:     config.Logger.File.MaxAge,
			LocalTime:  config.Logger.File.LocalTime,
			Compress:   config.Logger.File.Compress,
		})
	}

	if config.Logger.File.Filename == "" && config.Logger.Elastic.Address != "" && config.Logger.Elastic.Index != "" {
		hook, err := createElasticHook(config.Logger.Elastic)

		if err != nil {
			return nil, &LogElasticInitializationError{
				Address: config.Logger.Elastic.Address,
				Cause:   err,
			}
		}

		log.AddHook(hook)
	}

	return LogrusLogger{
		logger: log,
		config: *config,
	}, nil
}

func (l LogrusLogger) Debugf(format string, args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Debugf(format, args...)
}

func (l LogrusLogger) Infof(format string, args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Infof(format, args...)
}

func (l LogrusLogger) Warnf(format string, args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Warnf(format, args...)
}

func (l LogrusLogger) Errorf(format string, args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Errorf(format, args...)
}

func (l LogrusLogger) Fatalf(format string, args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Fatalf(format, args...)
}

func (l LogrusLogger) Debug(args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Debug(args...)
}

func (l LogrusLogger) Info(args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Info(args...)
}

func (l LogrusLogger) Warn(args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Warn(args...)
}

func (l LogrusLogger) Error(args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Error(args...)
}

func (l LogrusLogger) Fatal(args ...interface{}) {
	l.logger.WithFields(logrus.Fields{
		"name": l.config.Logger.Name,
	}).Fatal(args...)
}
