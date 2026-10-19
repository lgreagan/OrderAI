// 授权码生成器：给另一台服务器出授权码用。
//
//	go run ./tools/licgen                                  # 输出本机指纹 + 本机授权码
//	go run ./tools/licgen -cpu BFEBFBFF000906EA -mac 00:1A:2B:3C:4D:5E   # 给指定机器出码
//
// 密钥与 main.go 里的 licenseSecret 必须一致，改了这里要同步改 main.go。
package main

import (
	"flag"
	"fmt"

	"OrderAI/license"
)

// 与 main.go 的 licenseSecret 保持一致。
const secret = "OrderAI!2026@根#license$7f3a9c"

func main() {
	cpu := flag.String("cpu", "", "目标机器 CPU ID")
	mac := flag.String("mac", "", "目标机器主网卡 MAC")
	flag.Parse()

	if *cpu == "" || *mac == "" {
		localCPU, localMAC, err := license.Fingerprint()
		if err != nil {
			fmt.Println("读取本机指纹失败:", err)
			return
		}
		fmt.Printf("本机 CPU : %s\n", localCPU)
		fmt.Printf("本机 MAC : %s\n", localMAC)
		fmt.Printf("授权码    : %s\n", license.Code(secret, localCPU, localMAC))
		fmt.Println()
		fmt.Println("（其它网卡，换网卡时从这里挑）")
		for _, m := range license.AllMACs() {
			fmt.Println("  ", m)
		}
		fmt.Println()
		fmt.Println("填到 main.go：")
		fmt.Printf("const licenseCode = %q\n", license.Code(secret, localCPU, localMAC))
		return
	}

	code := license.Code(secret, *cpu, *mac)
	fmt.Printf("CPU : %s\n", *cpu)
	fmt.Printf("MAC : %s\n", *mac)
	fmt.Printf("授权码: %s\n", code)
	fmt.Printf("const licenseCode = %q\n", code)
}
