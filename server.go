package main


import (
        "net"
        "flag"
        "fmt"
	"io"
	"log"
	"strings"
	"strconv"
	"time"
	"./utilization"
)

func checke(e error) {
	if (e != nil) {
		log.Fatal(e)
	}
}

var verbose int

func main() {
	buffer := make([]byte, 1024);
	fmt.Println("Listening");
	server, e := net.Listen("tcp", ":8192");
	flag.IntVar(&verbose, "v", 0, "verbose level");
	flag.Parse();

	checke(e)
	for (true) {
		client, e := server.Accept();
		checke(e)
		length, e := client.Read(buffer);
		checke(e)
		if (verbose > 0) {
			fmt.Println("Client gave us ", length,
				" bytes and said ", string(buffer));
		}
		args := strings.SplitAfter(string(buffer), ":")
		for i, arg := range args {
			args[i] = strings.Replace(arg, ":", "", -1)
		}
		msglen, e := strconv.Atoi(args[3]);
		checke(e)
		fmt.Printf("msglen = %d, txlen = %s\n", msglen, args[4]);
		switch args[2] {
		case "stream": go rx_stream(client, args[1], msglen)
		case "rr" : {
			txlen, e := strconv.Atoi(args[4]);
			checke(e)
			go rx_rr(client, args[1], msglen, txlen)
		}
		default:  return
		}
	}
}

func rx_stream(client net.Conn, proto string, msglen int) {
	buffer := make([]byte, msglen);
	server, e := net.Listen("tcp", ":0");
	checke(e)

	fmt.Println("stream listen on", server.Addr());
	ports := strings.SplitAfter((server.Addr().String()), ":")
	client.Write([]byte(ports[len(ports) - 1]))
	p, e := server.Accept();
	checke(e)
	length, e  := p.Read(buffer)
	startns := time.Now().UnixNano();
	cpu_before, e := utilization.Read_cpu()
	checke(e)
	bytes := int64(0);
	messages := int64(0);
	for (e == nil) {
		bytes = bytes + int64(length);
		messages++;
		length, e  = p.Read(buffer)
	}
	if (e != io.EOF) {
		log.Fatal(e);
	}
	elapsedns := time.Now().UnixNano() - startns;
	cpu_after, e := utilization.Read_cpu()
	checke(e)
	bandwidth := float64(bytes) / (float64(elapsedns) /  float64(1000 * 1000 * 1000));
	bandwidth = bandwidth * 8.0 / (1000.0 * 1000.0)
	if (verbose > 0) {
		fmt.Println("Elapsted time is ", elapsedns);
		fmt.Println("Read ", messages, " messages and ", bytes, " bytes");
		fmt.Println("Bandwidth is", bandwidth, "Mb/s");
	}
	u, ncpu := utilization.Calc_cpu(string(cpu_before), string(cpu_after))
	results := fmt.Sprintf("%s:%d:%f:%d:%d:%d:eof\n", "goodbye", ncpu, u, elapsedns, bytes, messages);
	fmt.Println("Writing:  ", results)
	n, e := client.Write([]byte(results))
	fmt.Println("Wrote ", n, " bytes, with error", e)
}

func rx_rr(client net.Conn, proto string, msglen int, txlen int) {
	buffer := make([]byte, msglen);
	txbuffer := make([]byte, txlen);
	server, e := net.Listen("tcp", ":0");
	checke(e)

	if (verbose > 1) {
		fmt.Println("rr listen on", server.Addr());
	}
	ports := strings.SplitAfter((server.Addr().String()), ":")
	client.Write([]byte(ports[len(ports) - 1]))
	p, e := server.Accept();
	length, e  := p.Read(buffer)
	cpu_before, e := utilization.Read_cpu()
	startns := time.Now().UnixNano();
	bytes := int64(0);
	messages := int64(0);
	for (e == nil) {
		length, e = p.Write(txbuffer)
		if (e != nil) {
			break;
		}
		bytes = bytes + int64(length);
		messages++;
		length, e  = p.Read(buffer)
		if (e != nil) {
			break;
		}
	}
	if (e != io.EOF) {
		fmt.Println("err =", e)
		log.Fatal(e);
	}
	elapsedns := time.Now().UnixNano() - startns;
	cpu_after, e := utilization.Read_cpu()
	checke(e)
	bandwidth := float64(bytes) / (float64(elapsedns) /  float64(1000 * 1000 * 1000));
	bandwidth = bandwidth * 8.0 / (1000.0 * 1000.0)
	if (verbose > 0) {
		fmt.Println("Elapsted time is ", elapsedns);
		fmt.Println("Read ", messages, " messages and ", bytes, " bytes");
		fmt.Println("Bandwidth is", bandwidth, "Mb/s");
	}
	u, ncpu := utilization.Calc_cpu(string(cpu_before), string(cpu_after))
	results := fmt.Sprintf("%s:%d:%f:%d:%d:%d:eof\n", "goodbye", ncpu, u, elapsedns, bytes, messages);
	fmt.Println("Writing:  ", results)
	n, e := client.Write([]byte(results))
	fmt.Println("Wrote ", n, " bytes, with error", e)
}

