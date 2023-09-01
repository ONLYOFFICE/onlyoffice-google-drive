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

package shared

var GdriveMimeOnlyofficeExtension map[string]string = map[string]string{
	"application/vnd.google-apps.document":     "docx",
	"application/vnd.google-apps.spreadsheet":  "xlsx",
	"application/vnd.google-apps.presentation": "pptx",
}

var GdriveMimeOnlyofficeMime map[string]string = map[string]string{
	"application/vnd.google-apps.document":     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"application/vnd.google-apps.spreadsheet":  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/vnd.google-apps.presentation": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

var CreateFileMapper map[string]string = map[string]string{
	"en":    "en-US",
	"de":    "de-DE",
	"es":    "es-ES",
	"fr":    "fr-FR",
	"it":    "it-IT",
	"ja":    "ja-JP",
	"pt-BR": "pt-BR",
	"pt-PT": "pt-PT",
	"ru":    "ru-RU",
	"zh":    "zh-CN",
}
