package internal

import (
	"fmt"
	"strings"
)

// displayWidth 兼容中英文宽度
func DisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 127 {
			w += 2 // 粗略认为中文宽度为2
		} else {
			w++
		}
	}
	return w
}

// Spaces 返回 n 个空格
func Spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%*s", n, "")
}

// 合并默认参数和额外参数
func MergeArgs(defaults, extras []string) []string {
	return append(defaults, extras...)
}

// joinArgs 将参数数组拼接为空格分隔字符串
func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.Join(args, " ")
}
