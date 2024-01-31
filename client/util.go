package main

import (
	"fmt"
	"math/rand"
	"os"
	"text/tabwriter"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// 随机生成文本颜色
func randomTextColor() string {

	// 定义颜色范围
	min := 45
	max := 240

	// 生成随机颜色值
	red := rand.Intn(max-min) + min
	green := rand.Intn(max-min) + min
	blue := rand.Intn(max-min) + min

	// 返回True Color格式的颜色字符串
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", red, green, blue)
}

func colouration(colorCode, content string) string {
	return colorCode + content + "\033[0m"
}

func formatPersonal(id int64, name string, seconds int64) string {
	t := time.Unix(seconds, 0).Format("15:04:05")
	return fmt.Sprintf("%s(%d)-%s", name, id, t)
}

func formatPersonalToPeer(id int64, name string, seconds int64) string {
	t := time.Unix(seconds, 0).Format("15:04:05")
	return fmt.Sprintf("->%s(%d)-%s", name, id, t)
}

func formatPersonalToWorld(seconds int64) string {
	t := time.Unix(seconds, 0).Format("15:04:05")
	return fmt.Sprintf("->[世界频道]-%s", t)
}

func formatPersonalWorld(id int64, name string, seconds int64) string {
	t := time.Unix(seconds, 0).Format("15:04:05")
	return fmt.Sprintf("[世界频道]%s(%d)-%s", name, id, t)
}

func formatGroup(id, group int64, name, groupName string, seconds int64) string {
	t := time.Unix(seconds, 0).Format("15:04:05")
	return fmt.Sprintf("%s(%d)[%s-%d]-%s", name, id, groupName, group, t)
}

func formatGroupToSend(group int64, groupName string, seconds int64) string {
	t := time.Unix(seconds, 0).Format("15:04:05")
	return fmt.Sprintf("->%s[%d]-%s", groupName, group, t)
}

func msgPrintPeer(personal, content string) {
	fmt.Println(personal)
	formatContent := fmt.Sprintf("\033[38;2;20;160;43m: \033[0m%s", content)
	fmt.Println(formatContent)
}

func msgPrintOwn(personal, content string) {
	w := tabwriter.NewWriter(os.Stdout, 15, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "\t\t\t"+personal)
	formatContent := fmt.Sprintf("\033[38;2;20;160;43m: \033[0m%s", content)
	fmt.Fprintln(w, "\t\t\t"+formatContent)
	w.Flush()
}

func msgPrintStatus(op int, id int64, name, desc string) {
	personal := formatPersonal(id, name, time.Now().Unix())
	var colorCode string
	if op == 0 {
		personal += " " + desc + "成功"
		colorCode = "\033[32m"
	} else {
		personal += " " + desc + "失败"
		colorCode = "\033[31m"
	}

	msg := colouration(colorCode, personal)
	fmt.Println(msg)
}
