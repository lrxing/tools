package tools

import (
	"math/rand"
	"time"
	"unsafe"
)

// 参考: https://www.flysnow.org/2019/09/30/how-to-generate-a-random-string-of-a-fixed-length-in-go.html

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var randc = rand.New(rand.NewSource(time.Now().UnixNano()))

// GenVerificationCodeN 生成指定字符长度的随机数据
// codeLen int 指定待生成字符长度，如果要生成6位字符就设置其长度为6
// customStrs []string 可变字符串， 用来指定用户自定义的字符集，如果不传自定义字符集
// 如：647048
func GenVerificationCodeN(codeLen int, customStrs ...string) string {
	charDict := letterBytes
	if len(customStrs) > 0 && customStrs[0] != "" {
		charDict = customStrs[0]
	}

	codeBytes := make([]byte, codeLen)
	for i, cache, remain := codeLen-1, randc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(charDict) {
			codeBytes[i] = charDict[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&codeBytes))
}

