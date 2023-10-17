package main

import (
	"fmt"
	"github.com/ahmetson/service-lib"
)

func main() {
	proxy, err := service.NewProxy()
	if err != nil {
		panic(err)
	}

	fmt.Println("starting proxy_1...")
	wg, err := proxy.Start()
	if err != nil {
		panic(err)
	}

	fmt.Println("proxy_1 waits for end of the proxy...")
	wg.Wait()

	fmt.Println("proxy_1 end")
}
