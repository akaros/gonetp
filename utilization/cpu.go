package utilization

import (
	"fmt"
	"io/ioutil"
	"strings"
)


type akaros_cpu_usage struct {
	irq, kern, user, idle float64
}

func read_akaros_cpu(line string, l *akaros_cpu_usage) {
	var dontcarei int
	fmt.Sscanf(line, "%d: %f ( %d%%), %f ( %d%%), %f ( %d%%), %f ( %d%%)",
		&dontcarei,
		&l.irq, &dontcarei,
		&l.kern, &dontcarei,
		&l.user, &dontcarei,
		&l.idle, &dontcarei)
}

func Calc_akaros_cpu(before string, after string) (utilized float64, num_cpu int) {
	num_cpu = strings.Count(before, ":") - 1;
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
	utilized = 100.0 * (busy / total_tick);
//	fmt.Println("cpu count is ", num_cpu, "idle=", idle, " busy=", busy);
//	fmt.Printf("cpu utilized = %3.2f\n", utilized);
	return;
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

func Calc_linux_cpu(before string, after string) (utilized float64, num_cpu int) {
	num_cpu = strings.Count(before, "cpu") - 1;
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
		if (false && i == 0) {
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
	// fmt.Println("cpu count is ", num_cpu, "idle=", idle, " busy=", busy);
	total_tick := float64(idle + busy);
	utilized = 100.0 * (float64(busy) / total_tick);
	// fmt.Printf("cpu utilized = %3.2f\n", utilized);
	return
}


func Calc_cpu(before string, after string) (cpu float64, num_cpu int) {
	_, e := ioutil.ReadFile("/proc/stat")
	if (e == nil) {
		cpu, num_cpu = Calc_linux_cpu(before, after)
	} else {
		cpu, num_cpu = Calc_akaros_cpu(before, after)
	}
	return
}

func Read_cpu() (cpu string, e error) {
	c, e := ioutil.ReadFile("/proc/stat")
	if (e != nil) {
		c, e = ioutil.ReadFile("/prof/mpstat")
	}
	cpu = string(c)
	return
}
