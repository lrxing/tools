// File Name: gac.go
// Author: liruixing
// E-mail: liruixing@sogou-inc.com
// Created Time: Mon Aug 30 10:45:01 2021

package examples

import (
        "strings"
        "fmt"

)

const (
        umisWordSeparator = "|"
        umisSegSeparator  = "\t"
        umisLineSeparator = "\n"
        umisBlockMark     = "decontrol"    // 不管控
        fullMatchMark     = "accurate"     // 精准匹配
        suspicLevel       = "suspic-level" // 敏感-suspic-level
        forbidLevel       = "delete"       // 删除-delete
)

type umisSensitiveFilter struct {
        Automation     *Automation
        FullMatchWords map[string]int
        TokensOfLine   []uint8
        Tokens         [][]string
        LineWordMap    map[string]map[int]interface{}
        WordMap        map[string]*wordObj
}

type wordObj struct {
        Key         string // 关键词
        Policy      string // 审核策略
        MatchPolicy string // 匹配策略
}

type hitInfo struct {
        HitWord string   // 命中的是敏感词中key中的哪个具体的词
        KeyInfo wordObj // 命中的是敏感词中的key的信息
}

// IsUmisWords 是否有敏感词
// @return []hitInfo 返回匹配项列表，当包含禁发内容时可能不是全部匹配项
// @return bool 是否有禁发敏感词 true:有禁发项敏感词
// @return bool 是否有敏感词 true:有敏感词，可能是禁发敏感词也可能是普通敏感词
func (u *umisSensitiveFilter) IsUmisWords(seq []rune) ([]hitInfo, bool, bool) {
        hitList, hit := hitUmisWords(u, seq)
        if hit == false {
                return nil, false, false
        }
        for _, h := range hitList {
                fmt.Printf("%v\n", h)
                if h.KeyInfo.IsForbidLevel() {
                        return hitList, true, true
                }
        }
        return hitList, false, true
}

func parseUmisConfig(remoteResponse []byte) *umisSensitiveFilter {
        lines := strings.Split(string(remoteResponse), umisLineSeparator)
        filter := new(umisSensitiveFilter)
        filter.Automation = GenAutomation()
        filter.TokensOfLine = make([]uint8, len(lines))
        filter.Tokens = make([][]string, len(lines))
        filter.LineWordMap = make(map[string]map[int]interface{})
        filter.WordMap = map[string]*wordObj{}
        filter.FullMatchWords = map[string]int{}
        for lineIndex, line := range lines {
                // 敏感词词表中一行表示一个敏感词或一组敏感词，多个敏感词使用“|”分割
                // 第一列为敏感词，第二列为审核策略，第三列为匹配策略，列之间使用\t分割
                segs := strings.Split(line, umisSegSeparator)
                if len(segs) <= 2 {
                        continue
                }
                tmpWordObj := new(wordObj)
                tmpWordObj.Key = segs[0]
                tmpWordObj.Policy = segs[1]
                tmpWordObj.MatchPolicy = segs[2]
                filter.WordMap[tmpWordObj.Key] = tmpWordObj
                if tmpWordObj.MatchPolicy == fullMatchMark {
                        filter.FullMatchWords[tmpWordObj.Key] = 1
                        continue
                }
                if tmpWordObj.MatchPolicy != umisBlockMark {
                        // 把多个词切开
                        filter.Tokens[lineIndex] = strings.Split(segs[0], umisWordSeparator)
                        var count uint8
                        for _, word := range filter.Tokens[lineIndex] {
                                // 为空不计
                                if word == "" {
                                        continue
                                }
                                count++
                                if _, ok := filter.LineWordMap[word]; !ok {
                                        filter.LineWordMap[word] = make(map[int]interface{})
                                }
                                filter.LineWordMap[word][lineIndex] = nil
                                filter.Automation.Insert([]rune(word), nil)
                        }
                        filter.TokensOfLine[lineIndex] = count
                }
        }
        filter.Automation.Compile()
        return filter
}

func hitUmisWords(filter *umisSensitiveFilter, seq []rune) ([]hitInfo, bool) {
        var hitList []hitInfo
        if filter.FullMatchWords[string(seq)] == 1 {
                if word, ok := filter.WordMap[string(seq)]; ok {
                        hitList = append(hitList, hitInfo{
                                HitWord: string(seq),
                                KeyInfo: *word,
                        })
                        return hitList, true
                }
                return hitList, false
        }
        matcher := filter.Automation
        matchRes := matcher.Match(seq)
        defer filter.Automation.PoolPut(matchRes)
        if matchRes.Len == 0 {
                return nil, false
        }
        lineInfo := make(map[int]uint8)
        // 保存已经处理过的词
        keys := make(map[string]bool)
        found := false
        var hitStrings []string
        for item := 0; item < matchRes.Len; item++ {
                keyBytes, _ := filter.Automation.GetMatched(matchRes.Indexes[item])
                key := string(keyBytes)
                // 如果已经处理过了，不用处理了
                if _, ok := keys[key]; ok {
                        continue
                }
                keys[key] = true
                if values, ok := filter.LineWordMap[key]; ok && values != nil {
                        for k := range values {
                                // 下面的循环用来做多词匹配的
                                // 比如词条“八婆|死肥猪”
                                // 那么待审核的文本中必须同时包含“八婆”和“死肥猪”才算命中
                                lineInfo[k]++
                                if filter.TokensOfLine[k] <= lineInfo[k] {
                                        hitStrings = filter.Tokens[k]
                                        if word, ok := filter.WordMap[strings.Join(hitStrings, "|")]; ok {
                                                h := hitInfo{
                                                        HitWord: key,
                                                        KeyInfo: *word,
                                                }
                                                hitList = append(hitList, h)
                                                found = true
                                        }
                                }
                        }
                }
        }
        return hitList, found
}

func (w *wordObj) IsForbidLevel() bool {
        if w.Policy == forbidLevel {
                return true
        }
        return false
}

func main() {
        remoteResponse := []byte(`长胖了|八百斤|千金小姐        suspic-level    fuzzy
死肥猪|八婆     suspic-level    fuzzy
大胖子  suspic-level    fuzzy`)
        umis := parseUmisConfig(remoteResponse)
        testList := []string{"小学1年级的时候八百斤，初中2年级","你就是个八婆？", "好好学", "大胖子","八婆今天买了一头死肥猪，特别肥"}
        for _, w := range testList {
                fmt.Printf("audit: %s ----- ", w)
                hitList, forbid, suspic := umis.IsUmisWords([]rune(w))
                if suspic {
                        fmt.Printf(" Hit")
                        if forbid {
                                fmt.Printf(" and has forbid words")
                        }
                        for _, ww := range hitList {
                                fmt.Printf(" [word: %s, key: %s, policy: %s]\t", ww.HitWord, ww.KeyInfo.Key, ww.KeyInfo.Policy)
                        }
                } else {
                        fmt.Printf(" Not Hit")
                }
                fmt.Println("")
        }
}
