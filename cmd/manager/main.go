package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/integr8ly/integreatly-operator/pkg/providers"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/integr8ly/integreatly-operator/pkg/apis"
	"github.com/integr8ly/integreatly-operator/pkg/controller"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
	products    []string
)
var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.StringSliceVarP(&products, "products", "p", []string{"all"}, "--products=rhsso,fuse")

	pflag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.Logger())

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	ctx := context.TODO()

	// Become the leader before proceeding
	err = leader.Become(ctx, "integreatly-operator-lock")
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace:          namespace,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr, products); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Create Service object to expose the metrics port.
	_, err = metrics.ExposeMetricsPort(ctx, metricsPort)
	if err != nil {
		log.Info(err.Error())
	}

	log.Info("Starting the Cmd.")

	providerFactory := providers.CloudProviderFactory{}
	provider, err := providerFactory.Get("aws")
	if err != nil{
		panic("unable to get cloud provider " + err.Error())
	}
	var bucket = "test-operator-bucket"
	if err := provider.CreateCloudStorage(bucket); err != nil{
		logrus.Error("aws error: ", err)
	}
	logrus.Info("created bucket ")
	if err := provider.ListCloudStorage(); err != nil{
		logrus.Error("aws error list: ", err)
	}
	logrus.Info("listed bucket ")
	if err := provider.RemoveCloudStorage( bucket); err != nil{
		logrus.Error("aws error: ", err)
	}
	logrus.Info("deleted bucket ")
	cloudCache := providers.CloudCache{
		ClusterName:"mycluster",
		Engine:providers.CloudCacheEngineRedis,
		EngineVersion:"3.2.4",
		Type: providers.CloudCacheTypeDev,
	}
	fmt.Println("creating elastic cache")
	if _, err := provider.CreateCloudCache(cloudCache); err != nil{
		logrus.Error("aws error create elasticache : ", err)
	}
	fmt.Println("removing elastic cache")
	if err := provider.RemoveCloudCache(cloudCache); err != nil{
		logrus.Error("aws error create elasticache : ", err)
	}
	cloudDB := providers.CloudDB{
		ClusterName:"mycluster",
		Type: providers.CloudDBTypeDev,
		EngineVersion:"9.6", DBName:"mydb",
		Engine:providers.CloudDbEnginePostgres,
		RetentionPeriod:7,
		StorageSize:50,
	}
	coords, err := provider.CreateCloudDB(cloudDB)
	if err != nil{
		logrus.Error("aws error create rds : ", err)
	}
	if coords != nil {
		fmt.Println("created rds instance " + coords.String())
	}
	if err := provider.RemoveCloudDB(cloudDB); err != nil{
		logrus.Error("aws error delete rds : ", err)
	}



	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}
