package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

var (
	n = flag.String("n", "smb-pi", "")
	d = flag.String("d", "", "")
	g = flag.String("g", "", "")
	u = flag.String("u", "", "")

	uid     int
	gid     int
	localIP net.IP
	err     error

	mntDir = "/mnt/usb"
	mntNfs = "/mnt/nfs"
)

var usage = `Usage: raspiNetworkShareSetup [options...]

Options:
	-n	share name. Defaults to "smb-pi"
	-d	usb drive path (required)
	-u	user id used for file ownership (required)
	-g	group id used for file ownership (required)

** should be run with root priviledges
`

var sambaConf = `[share]
Comment = %%name%%
Path = %%mnt%%
Browseable = yes
Writeable = Yes
only guest = no
create mask = 0777
directory mask = 0777
Public = yes
Guest ok = yes
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprint(usage))
	}
	flag.Parse()

	defer func() {
		if err != nil {
			fmt.Printf("error running raspiNetworkShareSetup: %v", err)
			os.Exit(1)
		}
	}()

	checkRequiredFlags()

	sudo := os.Getenv("SUDO_USER")
	if strings.TrimSpace(sudo) == "" {
		err = errors.New("this command should be run with root priviledges")
		return
	}

	err = setupIds()
	if err != nil {
		return
	}

	err = getLocalIpAddress()
	if err != nil {
		return
	}

	err = createMountDirectory()
	if err != nil {
		return
	}

	err = installRequiredSoftware()
	if err != nil {
		return
	}

	err = makeMntDirAccessible()
	if err != nil {
		return
	}

	println("Done.\nFinish the setup by restarting the computer with \"sudo reboot now\"")
}

func checkRequiredFlags() {
	if *u == "" || *g == "" || *d == "" {
		flag.Usage()
		os.Exit(0)
	}
}

func setupIds() error {
	fmt.Printf("looking up ids for user %s and group %s\n", *u, *g)
	group, err := user.LookupGroup(*g)
	if err != nil {
		return err
	}

	user, err := user.Lookup(*u)
	if err != nil {
		return err
	}

	gid, err = strconv.Atoi(group.Gid)
	if err != nil {
		return err
	}

	uid, err = strconv.Atoi(user.Uid)
	if err != nil {
		return err
	}

	return err
}

func getLocalIpAddress() error {
	fmt.Printf("getting local ip address\n")

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}

	var ip string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
			}
		}
	}

	if ip == "" {
		return errors.New("could not find the local ip address")
	}

	localIP = net.ParseIP(ip)

	return nil
}

func makeMntDirAccessible() error {
	fmt.Printf("using ip %s\n", localIP.String())

	ipv4Mask := net.CIDRMask(24, 32)
	ip := localIP.Mask(ipv4Mask).String() + "/24"
	expLine := mntDir + " " + ip + "(rw,sync)"

	fmt.Printf("MANUAL STEP: to make the mount accessible, append the following line to the /etc/exports file:\n%s\n", expLine)

	var c string
	for c != "y" {
		println("Input \"y\" to continue:")
		_, err = fmt.Scanf("%s", &c)
	}

	fmt.Printf("enabling rpcbind \n")
	cmd := exec.Command("update-rc.d", "rpcbind", "enable")
	err = cmd.Run()

	fmt.Printf("enabling nfs-common \n")
	cmd = exec.Command("update-rc.d", "nfs-common", "enable")
	err = cmd.Run()

	return nil
}

func setupSMBConf() error {
	fmt.Printf("setting up smb share\n")
	sambaConf = strings.Replace(sambaConf, "%%name%%", *n, -1)
	sambaConf = strings.Replace(sambaConf, "%%mnt%%", mntDir, -1)

	err := appendLineToFile("/etc/samba/smb.conf", sambaConf)
	if err != nil {
		return err
	}

	cmd := exec.Command("/etc/init.d/samba", "restart")
	output, err := cmd.Output()

	fmt.Printf("%s\n", output)

	return err
}

func createMountDirectory() error {
	fmt.Printf("creating mount directory using gid %d\n", gid)
	err := os.MkdirAll(mntDir, 1777)

	var p string
	for _, v := range strings.Split(mntDir, "/") {
		p = p + "/" + v
		err = os.Chown(p, -1, gid)
	}

	fstabLine := *d + " " + mntDir + " auto defaults,user 0 1"
	fmt.Printf("MANUAL STEP: to automatically mount the drive, append the following line to the /etc/fstab file:\n%s\n", fstabLine)

	var c string
	for c != "y" {
		println("Input \"y\" to continue:")
		_, err = fmt.Scanf("%s", &c)
	}

	return err
}

func installRequiredSoftware() error {
	fmt.Printf("installing required software\n")
	cmd := exec.Command("apt", "install", "-y", "nfs-server", "nfs-common", "autofs", "samba", "samba-common-bin")
	err := cmd.Run()

	return err
}

func appendLineToFile(file string, line string) error {
	if !fileExists(file) {
		_, err := os.Create(file)
		if err != nil {
			return errors.New(fmt.Sprintf("could not create file %s: %v", file, err.Error()))
		}
	}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return errors.New(fmt.Sprintf("could not read file: %s: %v", file, err.Error()))
	}

	s := string(b)
	if strings.Contains(s, line) {
		return nil
	}

	f, err := os.Open(file)
	if err != nil {
		return errors.New(fmt.Sprintf("could not open file: %s: %v", file, err.Error()))
	}

	defer f.Close()
	_, err = f.WriteString(line)
	if err != nil {
		return errors.New(fmt.Sprintf("could not write to file: %s: %v", file, err.Error()))
	}

	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
