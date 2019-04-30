package provisioner

import (
	"flag"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/klog/klogr"

	"github.com/yard-turkey/lib-bucket-provisioner/pkg/client/clientset/versioned"
	informers "github.com/yard-turkey/lib-bucket-provisioner/pkg/client/informers/externalversions"
	"github.com/yard-turkey/lib-bucket-provisioner/pkg/provisioner/api"
)

// Controller is the first iteration of our internal provisioning
// Controller.  The passed-in bucket provisioner, coded by the user of the
// library, is stored for later Provision and Delete calls.
type Provisioner struct {
	Name            string
	Provisioner     api.Provisioner
	claimController controller
	informerFactory informers.SharedInformerFactory
	// TODO context?
}

func initLoggers() {
	log = klogr.New().WithName(api.Domain + "/provisioner-manager")
	logD = log.V(1)
}

func initFlags() {
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)

	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		kflag := klogFlags.Lookup(f.Name)
		if kflag != nil {
			val := f.Value.String()
			kflag.Value.Set(val)
		}
	})
	if !flag.Parsed() {
		flag.Parse()
	}
}

// NewProvisioner should be called by importers of this library to
// instantiate a new provisioning Controller. This Controller will
// respond to Add / Update / Delete events by calling the passed-in
// provisioner's Provisioner and Delete methods.
// The Provisioner will be restrict to operating only to the namespace given
func NewProvisioner(
	cfg *rest.Config,
	provisionerName string,
	provisioner api.Provisioner,
	namespace string,
) (*Provisioner, error) {

	initFlags()
	initLoggers()

	libClientset := versioned.NewForConfigOrDie(cfg)
	clientset := kubernetes.NewForConfigOrDie(cfg)

	informerFactory := informers.NewSharedInformerFactory(libClientset, 0)

	p := &Provisioner{
		Name:            provisionerName,
		informerFactory: informerFactory,
		claimController: NewController(provisionerName, provisioner, clientset, libClientset,
			informerFactory.Objectbucket().V1alpha1().ObjectBucketClaims(),
			informerFactory.Objectbucket().V1alpha1().ObjectBuckets()),
	}

	return p, nil
}

// Run starts the claim and bucket controllers.
func (p *Provisioner) Run(stopCh <-chan struct{}) (err error) {
	defer klog.Flush()
	log.Info("starting provisioner Controller")

	p.informerFactory.Start(stopCh)

	go func() {
		err = p.claimController.Start(stopCh)
	}()
	<-stopCh
	return
}
