package onlyoffice

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ErrOnlyofficeExtensionNotSupported = errors.New("file extension is not supported")
	ErrInvalidContentLength            = errors.New("could not perform api actions due to exceeding content-length")
)

const (
	_OnlyofficeWordType  string = "word"
	_OnlyofficeCellType  string = "cell"
	_OnlyofficeSlideType string = "slide"
)

var OnlyofficeEditableExtensions map[string]string = map[string]string{
	"docm":  _OnlyofficeWordType,
	"docx":  _OnlyofficeWordType,
	"docxf": _OnlyofficeWordType,
	"oform": _OnlyofficeWordType,
	"dotm":  _OnlyofficeWordType,
	"dotx":  _OnlyofficeWordType,
	"xlsm":  _OnlyofficeCellType,
	"xlsx":  _OnlyofficeCellType,
	"xltm":  _OnlyofficeCellType,
	"xltx":  _OnlyofficeCellType,
	"potm":  _OnlyofficeSlideType,
	"potx":  _OnlyofficeSlideType,
	"ppsm":  _OnlyofficeSlideType,
	"ppsx":  _OnlyofficeSlideType,
	"pptm":  _OnlyofficeSlideType,
	"pptx":  _OnlyofficeSlideType,
}

var OnlyofficeOOXMLEditableExtensions map[string]string = map[string]string{
	"doc":   _OnlyofficeWordType,
	"dot":   _OnlyofficeWordType,
	"fodt":  _OnlyofficeWordType,
	"mht":   _OnlyofficeWordType,
	"xml":   _OnlyofficeWordType,
	"sxw":   _OnlyofficeWordType,
	"stw":   _OnlyofficeWordType,
	"htm":   _OnlyofficeWordType,
	"mhtml": _OnlyofficeWordType,
	"wps":   _OnlyofficeWordType,
	"wpt":   _OnlyofficeWordType,
	"fods":  _OnlyofficeCellType,
	"xls":   _OnlyofficeCellType,
	"xlt":   _OnlyofficeCellType,
	"sxc":   _OnlyofficeCellType,
	"et":    _OnlyofficeCellType,
	"ett":   _OnlyofficeCellType,
	"xlsb":  _OnlyofficeCellType,
	"fodp":  _OnlyofficeSlideType,
	"pot":   _OnlyofficeSlideType,
	"pps":   _OnlyofficeSlideType,
	"ppt":   _OnlyofficeSlideType,
	"sxi":   _OnlyofficeSlideType,
	"dps":   _OnlyofficeSlideType,
	"dpt":   _OnlyofficeSlideType,
}

var OnlyofficeDataLossEditableExtensions map[string]string = map[string]string{
	"epub": _OnlyofficeWordType,
	"fb2":  _OnlyofficeWordType,
	"html": _OnlyofficeWordType,
	"odt":  _OnlyofficeWordType,
	"ott":  _OnlyofficeWordType,
	"rtf":  _OnlyofficeWordType,
	"txt":  _OnlyofficeWordType,
	"csv":  _OnlyofficeCellType,
	"ods":  _OnlyofficeCellType,
	"ots":  _OnlyofficeCellType,
	"odp":  _OnlyofficeSlideType,
	"otp":  _OnlyofficeSlideType,
}

var OnlyofficeViewOnlyExtensions map[string]string = map[string]string{
	"djvu": _OnlyofficeWordType,
	"oxps": _OnlyofficeWordType,
	"pdf":  _OnlyofficeWordType,
	"xps":  _OnlyofficeWordType,
}

type OnlyofficeFileUtility interface {
	ValidateFileSize(ctx context.Context, limit int64, url string) error
	EscapeFilename(filename string) string
	IsExtensionSupported(fileExt string) bool
	IsExtensionEditable(fileExt string) bool
	IsExtensionViewOnly(fileExt string) bool
	IsExtensionLossEditable(fileExt string) bool
	IsExtensionOOXMLConvertable(fileExt string) bool
	GetFileType(fileExt string) (string, error)
	GetFileExt(filename string) string
}

func NewOnlyofficeFileUtility() OnlyofficeFileUtility {
	return fileUtility{}
}

type fileUtility struct{}

func (u fileUtility) ValidateFileSize(ctx context.Context, limit int64, url string) error {
	resp, err := http.Head(url)

	if err != nil {
		return err
	}

	if val, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 0); val > limit || err != nil {
		return ErrInvalidContentLength
	}

	return nil
}

func (u fileUtility) EscapeFilename(filename string) string {
	f := strings.ReplaceAll(filename, "\\", ":")
	f = strings.ReplaceAll(f, "/", ":")
	return f
}

func (u fileUtility) IsExtensionSupported(fileExt string) bool {
	ext := strings.ToLower(fileExt)
	if _, exists := OnlyofficeDataLossEditableExtensions[ext]; exists {
		return true
	}

	if _, exists := OnlyofficeEditableExtensions[ext]; exists {
		return true
	}

	if _, exists := OnlyofficeOOXMLEditableExtensions[ext]; exists {
		return true
	}

	if _, exists := OnlyofficeViewOnlyExtensions[ext]; exists {
		return true
	}

	return false
}

func (u fileUtility) IsExtensionEditable(fileExt string) bool {
	_, exists := OnlyofficeEditableExtensions[strings.ToLower(fileExt)]
	return exists
}

func (u fileUtility) IsExtensionViewOnly(fileExt string) bool {
	_, exists := OnlyofficeViewOnlyExtensions[strings.ToLower(fileExt)]
	return exists
}

func (u fileUtility) IsExtensionLossEditable(fileExt string) bool {
	_, exists := OnlyofficeDataLossEditableExtensions[strings.ToLower(fileExt)]
	return exists
}

func (u fileUtility) IsExtensionOOXMLConvertable(fileExt string) bool {
	_, exists := OnlyofficeOOXMLEditableExtensions[strings.ToLower(fileExt)]
	return exists
}

func (u fileUtility) GetFileType(fileExt string) (string, error) {
	ext := strings.ToLower(fileExt)
	if fType, exists := OnlyofficeEditableExtensions[ext]; exists {
		return fType, nil
	}

	if fType, exists := OnlyofficeDataLossEditableExtensions[ext]; exists {
		return fType, nil
	}

	if fType, exists := OnlyofficeOOXMLEditableExtensions[ext]; exists {
		return fType, nil
	}

	if fType, exists := OnlyofficeViewOnlyExtensions[ext]; exists {
		return fType, nil
	}

	return "", ErrOnlyofficeExtensionNotSupported
}

func (u fileUtility) GetFileExt(filename string) string {
	return strings.ReplaceAll(filepath.Ext(filename), ".", "")
}
