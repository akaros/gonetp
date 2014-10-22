package main


import (
	"flag"
	"fmt"
	"net"
	"log"
	"time"
//	"strconv"
	"strings"
	"io/ioutil"
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

type akaros_cpu_usage struct {
	irq, kern, user, idle float64
}

func read_akaros_cpu(line string, l *akaros_cpu_usage) {
	line = strings.TrimLeft(line, ":");
	fmt.Sscan(line, &l.irq);
	line = strings.TrimLeft(line, ",");
	fmt.Sscan(line, &l.kern);
	line = strings.TrimLeft(line, ",");
	fmt.Sscan(line, &l.user);
	line = strings.TrimLeft(line, ",");
	fmt.Sscan(line, &l.idle);
	fmt.Println("read ", l.irq, " ", l.kern, " ", l.user, " ", l.idle)
}

func calc_akaros_cpu(before string, after string) {
	num_cpu := strings.Count(before, ":") - 1;
	before_cpu := make([]akaros_cpu_usage, num_cpu)
	after_cpu := make([]akaros_cpu_usage, num_cpu)
	lines := strings.Split(string(before), "\n");
	for i, line := range lines {
		if (i == 0 || i > num_cpu) {
			continue;
		}
		read_akaros_cpu(line, &before_cpu[i - 1])
	}
	lines = strings.Split(string(after), "\n");
	for i, line := range lines {
		if (i == 0 || i > num_cpu) {
			continue;
		}
		read_akaros_cpu(line, &after_cpu[i - 1])
	}
	idle := float64(0.0);
	busy := float64(0.0);
	for i := 0; i < num_cpu; i++ {
		busy += after_cpu[i].irq - before_cpu[i].irq;
		busy += after_cpu[i].kern - before_cpu[i].kern;
		busy += after_cpu[i].user - before_cpu[i].user;
		idle += after_cpu[i].idle - before_cpu[i].idle;
	}
	total_tick := idle + busy;
	utilized := 100.0 * (busy / total_tick);
	fmt.Println("cpu count is ", num_cpu, "idle=", idle, " busy=", busy);
	fmt.Printf("cpu utilized = %3.2f%\n", utilized);
}

type linux_cpu_usage struct {
	user, nice, system, idle, iowait, irq, softirq, steal, guest, guest_nice uint64
}

func read_linux_cpu(line string, l *linux_cpu_usage) {
	var dontcare string
	n, e := fmt.Sscan(strings.TrimLeft(line, " "),
		&dontcare, &l.user, &l.nice, &l.system, &l.idle,
		&l.iowait, &l.irq, &l.softirq, &l.steal, &l.guest, &l.guest_nice);
	if (e != nil) {
		fmt.Println("read ", n, " items err = ", e, "idle was", l.idle)
	}
}

func linux_tick_subtract(start uint64, end uint64) (r uint64){
	r = end - start;
	if (end >= start ||
		0 != (start & (uint64(0xffffffff00000000)))) {
		return;
	}
	/* wrapped */
	r += uint64(0xffffffff00000000)
	return;
}

func calc_linux_cpu(before string, after string) {
	num_cpu := strings.Count(before, "cpu") - 1;
	before_cpu := make([]linux_cpu_usage, num_cpu)
	after_cpu := make([]linux_cpu_usage, num_cpu)
	lines := strings.Split(string(before), "\n");
	for i, line := range lines {
		if (i == 0 || i > num_cpu) {
			continue;
		}
		read_linux_cpu(line, &before_cpu[i - 1])
	}
	lines = strings.Split(string(after), "\n");
	for i, line := range lines {
		if (i == 0 || i > num_cpu) {
			continue;
		}
		read_linux_cpu(line, &after_cpu[i - 1])
	}
	idle := uint64(0);
	busy := uint64(0);
	for i := 0; i < num_cpu; i++ {
		idle += linux_tick_subtract(before_cpu[i].idle,
			after_cpu[i].idle);
		if (i == 0) {
			fmt.Println("idle ", idle, " before ", 
				before_cpu[i].idle, " after ",
				after_cpu[i].idle);
		}
		idle += linux_tick_subtract(before_cpu[i].iowait,
			after_cpu[i].iowait);
		busy += linux_tick_subtract(before_cpu[i].user,
			after_cpu[i].user);
		busy += linux_tick_subtract(before_cpu[i].nice,
			after_cpu[i].nice);
		busy += linux_tick_subtract(before_cpu[i].system,
			after_cpu[i].system);
		busy += linux_tick_subtract(before_cpu[i].irq,
			after_cpu[i].irq);
		busy += linux_tick_subtract(before_cpu[i].softirq,
			after_cpu[i].softirq);
		busy += linux_tick_subtract(before_cpu[i].steal,
			after_cpu[i].steal);
		busy += linux_tick_subtract(before_cpu[i].guest,
			after_cpu[i].guest);
		busy += linux_tick_subtract(before_cpu[i].guest_nice,
			after_cpu[i].guest_nice);
	}
	fmt.Println("cpu count is ", num_cpu, "idle=", idle, " busy=", busy);
	total_tick := float64(idle + busy);
	utilized := 100.0 * (float64(busy) / total_tick);
	fmt.Printf("cpu utilized = %3.2f\n", utilized);
}


func main() {
	stats_type := "none"
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
	cpu_before, e := ioutil.ReadFile("/proc/stat")
	if (e == nil) {
		stats_type = string("linux")
	} else {
		cpu_before, e = ioutil.ReadFile("/prof/mpstat")
		if (e == nil) {
			stats_type = string("akaros")
		}
	}
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
	switch(stats_type) {
	case "linux":
		cpu_after, e := ioutil.ReadFile("/proc/stat")
		checke(e)
		calc_linux_cpu(string(cpu_before), string(cpu_after))
	case "akaros":
		cpu_after, e := ioutil.ReadFile("/prof/mpstat")
		checke(e)
		calc_akaros_cpu(string(cpu_before), string(cpu_after))
	}

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
