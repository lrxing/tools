package tools

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/valyala/fasthttp"
)

// FileInfo 上传文件详情
type FileInfo struct {
	Size           int64  // 文件字节数
	Name           string // 带扩展名的文件名
	NameWithoutExt string // 不带扩展名的文件名
	Path           string // 文件存放的目录
	MD5            string // 文件的MD5值，用来判断文件是否有更改
	Ext            string // 扩展名
	Type           string // 文件类型 例如:image/jpeg
    FileContent    []byte // 存储字节序文件
}

type uploadConfig struct {
	outputType int             // 上传的文件以什么形式保存: 0:字节;1:文件
	maxSize    int64           // 最大支持的字节数
	extSets    map[string]bool // 支持的文件扩展名
	filePath   string          // 文件存储目录
}

type FileType struct {
	Type    string // 文件类型 例如:image/jpeg
	Ext     string // 扩展名
	Charset string // 字符集 文本类型需要此字段， 如:utf-8
}

const (
	saveAsBytes = iota // 0:字节;
	saveAsFile         // 1:文件;
)

var mimeType = map[string]string{
	"image/bmp":                "bmp",
	"image/gif":                "gif",
	"image/vnd.microsoft.icon": "ico",
	"image/jpeg":               "jpg",
	"image/png":                "png",
	"image/svg+xml":            "svg",
	"image/tiff":               "tiff",
	"image/webp":               "webp",
	"application/pdf":          "pdf",

	"bmp":  "image/bmp",
	"gif":  "image/gif",
	"ico":  "image/vnd.microsoft.icon",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"svg":  "image/svg+xml",
	"tiff": "image/tiff",
	"webp": "image/webp",
	"pdf":  "application/pdf",
}

func NewUploadConfig() uploadConfig {
	return uploadConfig{
		maxSize:  0,
		extSets:  map[string]bool{},
		filePath: "",
	}
}

func (c uploadConfig) SetMaxSize(size int64) uploadConfig {
	c.maxSize = size
	return c
}

func (c uploadConfig) AddFileType(fileType string) uploadConfig {
	c.extSets[fileType] = true
	return c
}

func (c uploadConfig) RemoveFileType(fileType string) uploadConfig {
	if _, ok := c.extSets[fileType]; ok {
		c.extSets[fileType] = false
	}
	return c
}

func (c uploadConfig) SaveAsFile(filePath string) uploadConfig {
	c.outputType = saveAsFile
	c.filePath = filePath
	return c
}

func (c uploadConfig) SaveAsBytes() uploadConfig {
	c.outputType = saveAsBytes
	return c
}

func (c uploadConfig) supportImgExt(ext string) bool {
	if yes, ok := c.extSets[ext]; ok && yes {
		return true
	}
	return false
}

func (c uploadConfig) supportImgSize(size int64) bool {
	if size <= c.maxSize {
		return true
	}
	return false
}

//UploadFile 上传小文件
func UploadFile(ctx *fasthttp.RequestCtx, upConfig uploadConfig, field string) (*FileInfo, error) {
	//根据参数名获取上传的文件
	fileHeader, err := ctx.FormFile(field)
	if err != nil {
		log.Printf("Fail to get upload file from formFile,Err: %s", err.Error())
		return nil, err
	}

	if ok := upConfig.supportImgSize(fileHeader.Size); !ok {
		return nil, fmt.Errorf("max file Size is %d", upConfig.maxSize)
	}

	//打开上传的文件
	fileIn, err := fileHeader.Open()
	if err != nil {
		log.Printf("Fail to open fileHeader,Err: %s", err.Error())
		return nil, err
	}

	buff := &bytes.Buffer{}
	_, err = io.Copy(buff, fileIn)
	//使用完关闭文件
	_ = fileIn.Close()

	if err != nil {
		log.Printf("Fail to copy file to buffer,Err: %s", err.Error())
		return nil, err
	}

	fileName, fileExt, err := parseFileName(fileHeader.Filename)
	if err != nil {
		log.Printf("Fail to parse file name,Err: %s", err.Error())
		return nil, err
	}

	fType, err := parseFileType(buff.Bytes())
	if err != nil {
		log.Printf("Fail to parse file type", err.Error())
		return nil, err
	}
	if fileExt == "" {
		fileExt = fType.Ext
	}

	if ok := upConfig.supportImgExt(fileExt); !ok {
		return nil, fmt.Errorf("not support this ext type")
	}

	//file MD5
	fileMD5 := MD5(buff.Bytes())

	fileInfo := &FileInfo{
		Name:           fileHeader.Filename,
		NameWithoutExt: fileName,
		Ext:            fileExt,
		Type:           fType.Type,
		Path:           upConfig.filePath,
		MD5:            fileMD5,
		Size:           fileHeader.Size,
		FileContent:    []byte{},
	}

	if upConfig.outputType == saveAsFile {
		if err := saveToFile(buff, upConfig.filePath, fileHeader.Filename, fileHeader.Size); err != nil {
			return nil, err
		}
	} else {
		fileInfo.FileContent = buff.Bytes()
	}
	return fileInfo, nil
}

func MD5(buff []byte) string {
    return fmt.Sprintf("%x", md5.Sum(buff))
}

func saveToFile(buff *bytes.Buffer, filePath, fileName string, fileSize int64) error {
	if err := os.MkdirAll(filePath, 777); err != nil {
		return err
	}

	targetFile := filePath + "/" + fileName

	nf, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Printf("Fail to create target file,Err: %s", err.Error())
		return err
	}
	//使用完需要关闭
	defer func() {
		_ = nf.Close()
	}()
	//复制文件内容
	writtenSize, err := io.Copy(nf, buff)
	if err != nil {
		log.Printf("Fail to copy file to target file,Err: %s", err.Error())
		return err
	}
	if fileSize != writtenSize {
		log.Printf("Fail to save complete file")
		return fmt.Errorf("file not complete")
	}
	return nil
}

// parseFileName 解析文件名
func parseFileName(fileFullName string) (fileName, fileExt string, err error) {
	if len(fileFullName) == 0 {
		return "", "", fmt.Errorf("fileName Is Empty")
	}

	idx := strings.LastIndex(fileFullName, "/")
	if idx != -1 {
		fileFullName = fileFullName[idx+1:]
	}

	idx = strings.LastIndex(fileFullName, ".")
	if idx == -1 {
		return fileFullName, "", nil
	}
	return Trim(fileFullName[:idx]), Trim(fileFullName[idx+1:]), nil
}

func Trim(str string) string {
	return strings.TrimFunc(str, func(r rune) bool {
		return unicode.IsSpace(r)
	})
}

func parseFileType(fileCon []byte) (fileType FileType, err error) {
	if fileCon == nil || len(fileCon) == 0 {
		return fileType, fmt.Errorf("fileEmpty")
	}

	conType := http.DetectContentType(fileCon)
	if conType == "" {
		return fileType, fmt.Errorf("file type is empty")
	}

	// 像 text/plain; charset=utf-8 这种需要把字符集去掉
	// 并且规范的 Content-Type 总是类型在前，字符集在后，中间用分号隔开
	if i := strings.Index(conType, ";"); i > 0 {
		if i+1 < len(conType) {
			fileType.Charset = Trim(conType[i+1:])
		}
		conType = conType[:i]
	}

	ext, found := mimeType[conType]
	if !found {
		return fileType, fmt.Errorf("err file type")
	}
	fileType.Type = conType
	fileType.Ext = ext
	return fileType, nil
}

