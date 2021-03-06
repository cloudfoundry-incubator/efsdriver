package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	cf_http "code.cloudfoundry.org/cfhttp"
	cf_debug_server "code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/dockerdriver/invoker"
	"code.cloudfoundry.org/efsdriver/efsmounter"
	"code.cloudfoundry.org/efsdriver/efsvoltools"
	"code.cloudfoundry.org/efsdriver/efsvoltools/voltoolshttp"
	"code.cloudfoundry.org/efsdriver/efsvoltools/voltoolslocal"
	"code.cloudfoundry.org/goshims/bufioshim"
	"code.cloudfoundry.org/goshims/filepathshim"
	"code.cloudfoundry.org/goshims/ioutilshim"
	"code.cloudfoundry.org/goshims/osshim"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/volumedriver"
	"code.cloudfoundry.org/volumedriver/mountchecker"
	"code.cloudfoundry.org/volumedriver/oshelper"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var atAddress = flag.String(
	"listenAddr",
	"127.0.0.1:9750",
	"host:port to serve volume management functions",
)

var efsVolToolsAddress = flag.String(
	"efsVolToolsAddr",
	"",
	"host:port to serve efs volume tools functions (for drivers colocated with the efs broker)",
)

var driversPath = flag.String(
	"driversPath",
	"",
	"Path to directory where drivers are installed",
)

var transport = flag.String(
	"transport",
	"tcp",
	"Transport protocol to transmit HTTP over",
)

var mountDir = flag.String(
	"mountDir",
	"/tmp/volumes",
	"Path to directory where fake volumes are created",
)

var requireSSL = flag.Bool(
	"requireSSL",
	false,
	"whether the fake driver should require ssl-secured communication",
)

var caFile = flag.String(
	"caFile",
	"",
	"the certificate authority public key file to use with ssl authentication",
)

var certFile = flag.String(
	"certFile",
	"",
	"the public key file to use with ssl authentication",
)

var keyFile = flag.String(
	"keyFile",
	"",
	"the private key file to use with ssl authentication",
)
var clientCertFile = flag.String(
	"clientCertFile",
	"",
	"the public key file to use with client ssl authentication",
)

var clientKeyFile = flag.String(
	"clientKeyFile",
	"",
	"the private key file to use with client ssl authentication",
)

var availabilityZone = flag.String(
	"availabilityZone",
	"",
	"the EC2 AZ that this driver is running in",
)

var insecureSkipVerify = flag.Bool(
	"insecureSkipVerify",
	false,
	"whether SSL communication should skip verification of server IP addresses in the certificate",
)

var uniqueVolumeIds = flag.Bool(
	"uniqueVolumeIds",
	false,
	"whether the EFS driver should opt-in to unique volumes",
)

const fsType = "nfs4"
const mountOptions = "vers=4.0,rsize=1048576,wsize=1048576,timeo=600,retrans=2,actimeo=0"

func main() {
	parseCommandLine()

	var localDriverServer ifrit.Runner

	logger, logTap := newLogger()
	logger.Info("start", lager.Data{"availability-zone": availabilityZone})
	defer logger.Info("end")

	mounter := efsmounter.NewEfsMounter(invoker.NewRealInvoker(), fsType, mountOptions, *availabilityZone)

	client := volumedriver.NewVolumeDriver(
		logger,
		&osshim.OsShim{},
		&filepathshim.FilepathShim{},
		&ioutilshim.IoutilShim{},
		mountchecker.NewChecker(&bufioshim.BufioShim{}, &osshim.OsShim{}),
		*mountDir,
		mounter,
		oshelper.NewOsHelper(),
	)

	efsvoltools := voltoolslocal.NewEfsVolToolsLocal(
		&osshim.OsShim{},
		&filepathshim.FilepathShim{},
		&ioutilshim.IoutilShim{},
		*mountDir,
		mounter,
	)

	if *transport == "tcp" {
		localDriverServer = createEfsDriverServer(logger, client, efsvoltools, *atAddress, *driversPath, false, *efsVolToolsAddress, false)
	} else if *transport == "tcp-json" {
		localDriverServer = createEfsDriverServer(logger, client, efsvoltools, *atAddress, *driversPath, true, *efsVolToolsAddress, *uniqueVolumeIds)
	} else {
		localDriverServer = createEfsDriverUnixServer(logger, client, *atAddress)
	}

	servers := grouper.Members{
		{Name: "localdriver-server", Runner: localDriverServer},
	}

	if dbgAddr := cf_debug_server.DebugAddress(flag.CommandLine); dbgAddr != "" {
		servers = append(grouper.Members{
			{Name: "debug-server", Runner: cf_debug_server.Runner(dbgAddr, logTap)},
		}, servers...)
	}

	process := ifrit.Invoke(processRunnerFor(servers))
	logger.Info("started")

	untilTerminated(logger, process)
}

func exitOnFailure(logger lager.Logger, err error) {
	if err != nil {
		logger.Error("fatal-err..aborting", err)
		panic(err.Error())
	}
}

func untilTerminated(logger lager.Logger, process ifrit.Process) {
	err := <-process.Wait()
	exitOnFailure(logger, err)
}

func processRunnerFor(servers grouper.Members) ifrit.Runner {
	return sigmon.New(grouper.NewOrdered(os.Interrupt, servers))
}

func createEfsDriverServer(logger lager.Logger, client dockerdriver.Driver, efsvoltools efsvoltools.VolTools, atAddress, driversPath string, jsonSpec bool, efsToolsAddress string, uniqueVolumeIds bool) ifrit.Runner {
	advertisedUrl := "http://" + atAddress
	logger.Info("writing-spec-file", lager.Data{"location": driversPath, "name": "efsdriver", "address": advertisedUrl, "unique-volume-ids": uniqueVolumeIds})
	if jsonSpec {
		driverJsonSpec := dockerdriver.DriverSpec{Name: "efsdriver", Address: advertisedUrl, UniqueVolumeIds: uniqueVolumeIds}

		if *requireSSL {
			absCaFile, err := filepath.Abs(*caFile)
			exitOnFailure(logger, err)
			absClientCertFile, err := filepath.Abs(*clientCertFile)
			exitOnFailure(logger, err)
			absClientKeyFile, err := filepath.Abs(*clientKeyFile)
			exitOnFailure(logger, err)
			driverJsonSpec.TLSConfig = &dockerdriver.TLSConfig{InsecureSkipVerify: *insecureSkipVerify, CAFile: absCaFile, CertFile: absClientCertFile, KeyFile: absClientKeyFile}
			driverJsonSpec.Address = "https://" + atAddress
		}

		jsonBytes, err := json.Marshal(driverJsonSpec)

		exitOnFailure(logger, err)
		err = dockerdriver.WriteDriverSpec(logger, driversPath, "efsdriver", "json", jsonBytes)
		exitOnFailure(logger, err)
	} else {
		err := dockerdriver.WriteDriverSpec(logger, driversPath, "efsdriver", "spec", []byte(advertisedUrl))
		exitOnFailure(logger, err)
	}

	handler, err := driverhttp.NewHandler(logger, client)
	exitOnFailure(logger, err)

	var server ifrit.Runner
	if *requireSSL {
		tlsConfig, err := cf_http.NewTLSConfig(*certFile, *keyFile, *caFile)
		if err != nil {
			logger.Fatal("tls-configuration-failed", err)
		}
		server = http_server.NewTLSServer(atAddress, handler, tlsConfig)
	} else {
		server = http_server.New(atAddress, handler)
	}

	if efsToolsAddress != "" {
		efsToolsHandler, err := voltoolshttp.NewHandler(logger, efsvoltools)
		exitOnFailure(logger, err)
		efsServer := http_server.New(efsToolsAddress, efsToolsHandler)
		server = grouper.NewParallel(os.Interrupt, grouper.Members{{Name: "dockerdriver", Runner: server}, {Name: "efstools", Runner: efsServer}})
	}

	return server
}

func createEfsDriverUnixServer(logger lager.Logger, client dockerdriver.Driver, atAddress string) ifrit.Runner {
	handler, err := driverhttp.NewHandler(logger, client)
	exitOnFailure(logger, err)
	return http_server.NewUnixServer(atAddress, handler)
}

func newLogger() (lager.Logger, *lager.ReconfigurableSink) {
	lagerConfig := lagerflags.ConfigFromFlags()
	lagerConfig.RedactSecrets = true

	return lagerflags.NewFromConfig("efs-driver-server", lagerConfig)
}

func parseCommandLine() {
	lagerflags.AddFlags(flag.CommandLine)
	cf_debug_server.AddFlags(flag.CommandLine)
	flag.Parse()
}
