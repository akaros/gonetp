package main


import (
	"flag"
	"fmt"
	"net"
	"log"
	"time"
	"strings"
	"./utilization"
)


func checke(e error) {
	if (e != nil) {
		log.Fatal(e)
	}
}


var host string
var test_type string
var remote_port string
var testlen int
var threadcnt int
var loc_rx_len int
var rem_rx_len int
var loc_tx_len int
var rem_tx_len int
var verbose int
var proto string
var ttype string

type test_results struct {
	elapsed int64;
	bytes int64;
	messages int64;
}

func main() {
	var tx_msglen string
	var rx_msglen string
	bw_chan := make(chan test_results)
	barrier := make(chan struct{})
	flag.StringVar(&test_type, "t", "TCP_STREAM", "test type");
	flag.StringVar(&host, "H", "", "Remote IP");
	flag.StringVar(&remote_port, "p", "8192", "Remote port");
	flag.IntVar(&testlen, "l", 10, "Test duration in seconds");
	flag.IntVar(&verbose, "v", 0, "verbose level");
	flag.IntVar(&threadcnt, "C", 1, "Concurrent threads");
	flag.StringVar(&tx_msglen, "m", "8192, 8192", "xmit message length");
	rem_tx_len = 8192
	flag.StringVar(&rx_msglen, "M", "8192, 8192", "rx message length");
	rem_rx_len = 8192
	flag.Parse();


	fmt.Sscanf(strings.Replace(tx_msglen, ",", " ", -1), "%d %d",
		&loc_tx_len, &rem_tx_len);


	fmt.Sscanf(strings.Replace(rx_msglen, ",", " ", -1), "%d %d",
		&loc_rx_len, &rem_rx_len);


	switch test_type {
	case "TCP_STREAM":
		proto = "tcp";
		ttype = "stream";
	case "TCP_RR":
		proto = "tcp";
		ttype = "rr";
	default:
		fmt.Println("Unsupported test ", test_type);
		return;
	}

	if (verbose > 0) {
		fmt.Println("args:");
		fmt.Println("\t t:", test_type);
		fmt.Println("\t H:", host);
		fmt.Println("\t p:", remote_port);
		fmt.Println("\t l:", testlen);
		fmt.Println("\t C:", threadcnt);
		fmt.Println("\t m:", tx_msglen);
		fmt.Println("\t M:", rx_msglen);
		fmt.Println("\t t:", test_type);
		fmt.Println("loc_tx_len=", loc_tx_len);
		fmt.Println("rem_tx_len=", rem_tx_len);
		fmt.Println("loc_rx_len=", loc_rx_len);
		fmt.Println("rem_rx_len=", rem_rx_len);
		fmt.Println("proto=", proto);
		fmt.Println("ttype=", ttype);
	}

	for i := 0; i < threadcnt; i++ {
		go worker(bw_chan, barrier);
	}
	time.Sleep(1 * time.Second)
	cpu_before, e := utilization.Read_cpu()
	close(barrier);
	tot := test_results{0, 0, 0}
	tot_msgs_per_sec := 0.0
	mb_per_sec := 0.0
	for i := 0; i < threadcnt; i++ {
		r := <- bw_chan
		tot.elapsed += r.elapsed
		tot.bytes += r.bytes
		tot.messages += r.messages
		secs := float64(r.elapsed) / (1000.0 * 1000.0 * 1000.0)
		mb_per_sec += ((float64(r.bytes) * 8.0) / (1000.0 * 1000.0)) / secs
		tot_msgs_per_sec += float64(r.messages) / secs;
	}
	cpu_after, e := utilization.Read_cpu()
	checke(e)
	utilization.Calc_cpu(string(cpu_before), string(cpu_after))
	secs := float64(tot.elapsed) / (1000.0 * 1000.0 * 1000.0);
	msgs_per_sec := float64(tot.messages) / secs;
	lat := 0.5 * (1.0 / msgs_per_sec) * 1000.0 * 1000.0
	fmt.Printf("Bandwidth is %6.2f Mb/s\n", mb_per_sec);
	fmt.Printf("Message rate is %6.2f Pkts/s\n", tot_msgs_per_sec);
	if (test_type == "TCP_RR") {
		fmt.Printf("Average Latency is %6.2f us/pkt\n", lat)
	}
}

func worker(bw_chan chan test_results, barrier chan struct{}) {
	handshake_buffer := make([]byte, 1024);
	if (verbose > 0) {
		fmt.Println("connecting to ", host + ":" + remote_port);
	}
	s, e := net.Dial("tcp", host + ":" + remote_port);
	if e != nil {
		log.Fatal(e)
	}
	/* say hello */
	server_arg := fmt.Sprintf("%s:%s:%s:%d:%d:eof\n",
		"hello", "tcp", ttype, rem_rx_len, rem_tx_len);
	s.Write([]byte(server_arg));
	/* read port to connect to */
	_, e = s.Read(handshake_buffer);
	switch test_type {
	case "TCP_STREAM":
		tcp_stream(bw_chan, barrier, string(handshake_buffer))
	case "TCP_RR":
		tcp_rr(bw_chan, barrier, string(handshake_buffer))
	}
}

func tcp_stream(bw_chan chan test_results, barrier chan struct{}, remote_port string) {
	b, e := net.Dial("tcp", host + ":" + remote_port)
	if e != nil {
		log.Fatal(e)
	}
	times_up := bool(false);
	tx_buffer := make([]byte, loc_tx_len);
	f := func () {
		times_up = true;
	}
	if (verbose > 1) {
		fmt.Println("waiting on barrier");
	}
	<-barrier;
	if (verbose > 1) {
		fmt.Println("barrier is now gone");
	}

	time.AfterFunc(time.Duration(testlen) * time.Second, f);
	bytes := int64(0);
	messages := int64(0);
	startns := time.Now().UnixNano();
	for (!times_up) {
		wrote, e := b.Write(tx_buffer);
		if e != nil {
			log.Fatal(e);
		}
		messages++;
		bytes += int64(wrote);
	}
	elapsedns := time.Now().UnixNano() - startns;
	bandwidth := float64(bytes) / (float64(elapsedns) /  float64(1000 * 1000 * 1000));
	bandwidth = bandwidth * 8.0 / (1000.0 * 1000.0)
	if (verbose > 0) {
		fmt.Println("Elapsted time is ", elapsedns);
		fmt.Println("Read ", messages, " messages and ", bytes, " bytes");
		fmt.Printf("Bandwidth is %6.2f Mb/s\n", bandwidth);
	}
	bw_chan <- test_results{elapsedns, bytes, messages}
}

func tcp_rr(bw_chan chan test_results, barrier chan struct{}, remote_port string) {
	b, e := net.Dial("tcp", host + ":" + remote_port)
	if e != nil {
		log.Fatal(e)
	}
	times_up := bool(false);
	rx_buffer := make([]byte, loc_rx_len);
	tx_buffer := make([]byte, loc_tx_len);
	f := func () {
		times_up = true;
	}
	if (verbose > 1) {
		fmt.Println("waiting on barrier");
	}
	<-barrier;
	if (verbose > 1) {
		fmt.Println("barrier is now gone");
	}

	time.AfterFunc(time.Duration(testlen) * time.Second, f);
	bytes := int64(0);
	messages := int64(0);
	startns := time.Now().UnixNano();
	for (!times_up) {
		wrote, e := b.Write(tx_buffer);
		if e != nil {
			log.Fatal(e);
		}
		messages++;
		bytes += int64(wrote);
		_, e  = b.Read(rx_buffer)
		if (e != nil) {
			break;
		}
	}
	elapsedns := time.Now().UnixNano() - startns;
	bandwidth := float64(bytes) / (float64(elapsedns) /  float64(1000 * 1000 * 1000));
	bandwidth = bandwidth * 8.0 / (1000.0 * 1000.0)
	if (verbose > 0) {
		fmt.Println("Elapsted time is ", elapsedns);
		fmt.Println("Read ", messages, " messages and ", bytes, " bytes");
		fmt.Printf("Bandwidth is %6.2f Mb/s\n", bandwidth);
	}
	bw_chan <- test_results{elapsedns, bytes, messages}
}
