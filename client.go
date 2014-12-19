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
var connected_udp int

type test_results struct {
	elapsed int64;
	bytes int64;
	messages int64;
	server_util float64;
	server_cpu_cnt int;
	server_elapsed int64;
	server_bytes int64;
	server_messages int64;
}

func main() {
	var tx_msglen string
	var rx_msglen string
	bw_chan := make(chan test_results)
	barrier := make(chan struct{})
	flag.StringVar(&test_type, "t", "TCP_STREAM", "test type");
	flag.StringVar(&host, "H", "127.0.0.1", "Remote IP");
	flag.StringVar(&remote_port, "p", "8192", "Remote port");
	flag.IntVar(&testlen, "l", 10, "Test duration in seconds");
	flag.IntVar(&verbose, "v", 0, "verbose level");
	flag.IntVar(&threadcnt, "C", 1, "Concurrent threads");
	flag.IntVar(&connected_udp, "N", 1, "Connected UDP");
	flag.StringVar(&tx_msglen, "m", "16384, 16384", "xmit message length");
	rem_tx_len = 16384
	flag.StringVar(&rx_msglen, "M", "16384, 16384", "rx message length");
	rem_rx_len = 16384
	flag.Parse();


	fmt.Sscanf(strings.Replace(tx_msglen, ",", " ", -1), "%d %d",
		&loc_tx_len, &rem_tx_len);


	fmt.Sscanf(strings.Replace(rx_msglen, ",", " ", -1), "%d %d",
		&loc_rx_len, &rem_rx_len);


	switch test_type {
	case "TCP_STREAM":
		proto = "tcp";
		ttype = "stream";
	case "UDP_STREAM":
		proto = "udp";
		ttype = "stream";
	case "TCP_RR":
		proto = "tcp";
		ttype = "rr";
		if (loc_tx_len != rem_rx_len) {
			fmt.Println("overriding rem rx len to ", loc_tx_len)
			rem_rx_len = loc_tx_len
		}
		if (loc_rx_len != rem_tx_len) {
			fmt.Println("overriding rem tx len to ", loc_rx_len)
			rem_tx_len = loc_rx_len
		}

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
	tot := test_results{0, 0, 0, 0, 0, 0, 0, 0}
	tot_msgs_per_sec := 0.0
	server_tot_msgs_per_sec := 0.0
	mb_per_sec := 0.0
	server_mb_per_sec := 0.0
	server_util := 0.0
	server_ncpu := 0
	for i := 0; i < threadcnt; i++ {
		r := <- bw_chan
		tot.elapsed += r.elapsed
		tot.bytes += r.bytes
		tot.messages += r.messages
		secs := float64(r.elapsed) / (1000.0 * 1000.0 * 1000.0)
		mb_per_sec += ((float64(r.bytes) * 8.0) / (1000.0 * 1000.0)) / secs
		tot_msgs_per_sec += float64(r.messages) / secs;

		tot.server_elapsed += r.server_elapsed
		tot.server_bytes += r.server_bytes
		tot.server_messages += r.server_messages
		secs = float64(r.server_elapsed) / (1000.0 * 1000.0 * 1000.0)
		server_mb_per_sec += ((float64(r.server_bytes) * 8.0) / (1000.0 * 1000.0)) / secs
		server_tot_msgs_per_sec += float64(r.server_messages) / secs;
		server_util += r.server_util;
		server_ncpu = r.server_cpu_cnt
	}
	cpu_after, e := utilization.Read_cpu()
	checke(e)
	u, ncpu := utilization.Calc_cpu(string(cpu_before), string(cpu_after))
	secs := float64(tot.elapsed) / (1000.0 * 1000.0 * 1000.0);
	msgs_per_sec := float64(tot.messages) / secs;
	lat := 0.5 * (1.0 / msgs_per_sec) * 1000.0 * 1000.0
	if (verbose > 1) {
		fmt.Printf("Bandwidth is %6.2f Mb/s\n", mb_per_sec);
		fmt.Printf("Message rate is %6.2f Pkts/s\n", tot_msgs_per_sec);
		fmt.Printf("CPU utilization is %6.2f of %d cores\n", u, ncpu)
		fmt.Printf("Server CPU utilization is %6.2f of %d cores\n", server_util / float64(threadcnt),
			server_ncpu)
		fmt.Printf("Server Bandwidth is %6.2f Mb/s\n", server_mb_per_sec);
		fmt.Printf("Server Message rate is %6.2f Pkts/s\n", server_tot_msgs_per_sec);
		if (test_type == "TCP_RR") {
			fmt.Printf("Average Latency is %6.2f us/pkt\n", lat)
		}
	}
	server_util = server_util / float64(threadcnt)
	switch test_type {
	case "TCP_STREAM": fallthrough;
	case "UDP_STREAM":
		fmt.Printf("rxmsg\ttxmsg\tElapsed\t\tUtilization\n")
		fmt.Printf("size\tsize\tTime\tTput\tlocal/remote")
		if (test_type == "UDP_STREAM") {
			fmt.Printf("\t\t     Messages")
		}
		fmt.Printf("\n")
		fmt.Printf("bytes\tbytes\tsecs\tMb/s\t%%cpu\t   #cores")
		if (test_type == "UDP_STREAM") {
			fmt.Printf("\t   OK\t\t   Error")
		}
		fmt.Printf("\n")
		fmt.Printf("\t%6d\t", loc_tx_len)
		fmt.Printf("%-6.2f\t", secs);
		fmt.Printf("%6.2f\t", mb_per_sec);
		fmt.Printf("%3.2f/", u);
		fmt.Printf("%3.2f ", server_util);
		fmt.Printf("%2.2f/", (u/100.0) * float64(ncpu));
		fmt.Printf("%2.2f\t", (server_util/100.0) * float64(server_ncpu));
		if (test_type == "UDP_STREAM") {
			fmt.Printf("%9d\t", tot.messages)
			fmt.Printf("%9d", 436453643)
		}
		fmt.Printf("\n")
		fmt.Printf("%6d",  rem_rx_len);
		fmt.Printf("\t\t")
		fmt.Printf("%-6.2f\t", secs);
		fmt.Printf("%6.2f\t", mb_per_sec);
		fmt.Printf("%3.2f/", u);
		fmt.Printf("%3.2f ", server_util);
		fmt.Printf("%2.2f/", (u/100.0) * float64(ncpu));
		fmt.Printf("%2.2f\t", (server_util/100.0) * float64(server_ncpu));
		if (test_type == "UDP_STREAM") {
			fmt.Printf("%9d\t", tot.messages)
			fmt.Printf("%9d", 436453643)
		}
		fmt.Printf("\n")
	case "TCP_RR":
		fmt.Printf("\tRecv\tSend\t\t\t\t\tUtilization\n")
		fmt.Printf("\tmsg\tmsg\tElapsed\n")
		fmt.Printf("thread\tsize\tsize\tTime\tThroughput\tlocal\tremote\tlocal\tremote\n")
		fmt.Printf("count\tbytes\tbytes\tsecs\tMsg/s\t\t%%cpu\t%%cpu\t#cores\t#cores\n")
		fmt.Printf("......\t......\t......\t......\t......\t\t......\t......\t......\t......\n")
		fmt.Printf("%6d\t", threadcnt)
		fmt.Printf("%6d\t%6d\t", loc_rx_len, loc_tx_len)
		fmt.Printf("%6.2f\t", secs);
		fmt.Printf("%9.2f\t", tot_msgs_per_sec)
		fmt.Printf("%6.2f\t", u);
		fmt.Printf("%6.2f\t", server_util);
		fmt.Printf("%6.2f\t", (u/100.0) * float64(ncpu));
		fmt.Printf("%6.2f\n", (server_util/100.0) * float64(server_ncpu));
		fmt.Printf("\t")
		fmt.Printf("%6.2fus 1/2RTT\n", lat)

	}
}

func worker(bw_chan chan test_results, barrier chan struct{}) {
	var s net.Conn;
	var e error
	var n int
	handshake_buffer := make([]byte, 1024);
	if (connected_udp != 0) {
		if (verbose > 0) {
			fmt.Println("connecting to ", host + ":" + remote_port);
		}
		s, e = net.Dial("tcp", host + ":" + remote_port);
		if e != nil {
			log.Fatal(e)
		}
		/* say hello */
		server_arg := fmt.Sprintf("%s:%s:%s:%d:%d:eof\n",
			"hello", "tcp", ttype, rem_rx_len, rem_tx_len);
		s.Write([]byte(server_arg));
		/* read port to connect to */
		n, e = s.Read(handshake_buffer);
	} else {
		handshake_buffer = []byte("9")
		n = 1
	}
	switch test_type {
	case "TCP_STREAM":
		tcp_stream(bw_chan, barrier, string(handshake_buffer[:n]), s)
	case "UDP_STREAM":
		udp_stream(bw_chan, barrier, string(handshake_buffer[:n]), s)
	case "TCP_RR":
		tcp_rr(bw_chan, barrier, string(handshake_buffer[:n]), s)
	}
}

func tcp_stream(bw_chan chan test_results, barrier chan struct{}, server string, s net.Conn) {
	var results test_results;
	b, e := net.Dial("tcp", host + ":" + server)
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
	results.bytes = int64(0);
	results.messages = int64(0);
	startns := time.Now().UnixNano();
	for (!times_up) {
		wrote, e := b.Write(tx_buffer);
		if e != nil {
			log.Fatal(e);
		}
		results.messages++;
		results.bytes += int64(wrote);
	}
	results.elapsed = time.Now().UnixNano() - startns;
	if (verbose > 0) {
		bandwidth := float64(results.bytes) / (float64(results.elapsed) /  float64(1000 * 1000 * 1000));
		bandwidth = bandwidth * 8.0 / (1000.0 * 1000.0)
		fmt.Println("Elapsted time is ", results.elapsed);
		fmt.Println("Wrote ", results.messages, " messages and ", results.bytes, " bytes");
		fmt.Printf("Bandwidth is %6.2f Mb/s\n", bandwidth);
	}
	b.Close();
	server_result := make([]byte, 1024)
	_, e = s.Read(server_result)
	checke(e)
	if (verbose > 0) {
		fmt.Println("server said", string(server_result))
	}
	fmt.Sscanf(string(server_result), "goodbye:%d:%f:%d:%d:%d",
		&results.server_cpu_cnt, &results.server_util, &results.server_elapsed,
		&results.server_bytes, &results.server_messages)
	bw_chan <- results
}

func udp_stream(bw_chan chan test_results, barrier chan struct{}, server string, s net.Conn) {
	var results test_results;
	var wrote int;
	var b *net.UDPConn
//	b, e := net.Dial("udp", host + ":" + server)
	ua, e := net.ResolveUDPAddr("udp",  host + ":" + server)
	if (connected_udp != 0) {
		b, e = net.DialUDP("udp", nil, ua)
	} else {
		b, e = net.ListenUDP("udp", nil)
	}
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
	results.bytes = int64(0);
	results.messages = int64(0);
	startns := time.Now().UnixNano();
	for (!times_up) {
		if (connected_udp == 0) {
			wrote, e = b.WriteToUDP(tx_buffer, ua)
		} else {
			wrote, e = b.Write(tx_buffer);
		}
		if e != nil {
			log.Fatal(e);
		}
		results.messages++;
		results.bytes += int64(wrote);
	}
	results.elapsed = time.Now().UnixNano() - startns;
	if (verbose > 0) {
		bandwidth := float64(results.bytes) / (float64(results.elapsed) /  float64(1000 * 1000 * 1000));
		bandwidth = bandwidth * 8.0 / (1000.0 * 1000.0)
		fmt.Println("Elapsted time is ", results.elapsed);
		fmt.Println("Wrote ", results.messages, " messages and ", results.bytes, " bytes");
		fmt.Printf("Bandwidth is %6.2f Mb/s\n", bandwidth);
	}
	b.Close();
	server_result := make([]byte, 1024)
	if (connected_udp == 1) {
		_, e = s.Read(server_result)
		checke(e)
	}
	if (verbose > 0) {
		fmt.Println("server said", string(server_result))
	}
	fmt.Sscanf(string(server_result), "goodbye:%d:%f:%d:%d:%d",
		&results.server_cpu_cnt, &results.server_util, &results.server_elapsed,
		&results.server_bytes, &results.server_messages)
	bw_chan <- results
}


func tcp_rr(bw_chan chan test_results, barrier chan struct{}, remote_port string, s net.Conn) {
	var results test_results;
	var rlen int
	var curlen int
	t,e := net.ResolveTCPAddr("tcp", host + ":" + remote_port)
	b, e := net.DialTCP("tcp", nil, t)
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
	results.bytes = int64(0);
	results.messages = int64(0);
	startns := time.Now().UnixNano();
	for (!times_up) {
		rlen = 0
		for (rlen != loc_rx_len) {
			curlen, e  = b.Read(rx_buffer)
			if (verbose > 3) {
				fmt.Println("message ", results.messages, "curlen=, ", curlen);
			}
			rlen += curlen
			if (rlen > loc_rx_len) {
				fmt.Println("message ", results.messages, "after read, ", curlen, rlen);
				log.Fatal(e)
			}
			if (e != nil) {
				break
			}
		}
		wrote, e := b.Write(tx_buffer);
		if e != nil {
			log.Fatal(e);
		}
		if (verbose > 3) {
				fmt.Println("message ", results.messages, "wrote=, ", wrote);
		}
		if (wrote != loc_tx_len) {
			fmt.Println("Short write! ", wrote)
		}
		results.messages++;
		results.bytes += int64(wrote);
		if (e != nil) {
			break;
		}
	}
	results.elapsed = time.Now().UnixNano() - startns;
	b.CloseWrite();
	server_result := make([]byte, 1024)
	_, e = s.Read(server_result)
	checke(e)
	if (verbose > 0) {
		bandwidth := float64(results.bytes) / (float64(results.elapsed) /  float64(1000 * 1000 * 1000));
		bandwidth = bandwidth * 8.0 / (1000.0 * 1000.0)
		fmt.Println("Elapsted time is ", results.elapsed);
		fmt.Println("Read ", results.messages, " messages and ", results.bytes, " bytes");
		fmt.Printf("Bandwidth is %6.2f Mb/s\n", bandwidth);
	}
	fmt.Sscanf(string(server_result), "goodbye:%d:%f:%d:%d:%d",
		&results.server_cpu_cnt, &results.server_util, &results.server_elapsed,
		&results.server_bytes, &results.server_messages)
	bw_chan <- results
}
