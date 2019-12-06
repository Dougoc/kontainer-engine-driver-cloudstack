package main

import (
	"context"
	"fmt"

	"github.com/rancher/kontainer-engine/drivers/options"
	"github.com/rancher/kontainer-engine/types"
	"github.com/xanzy/go-cloudstack/cloudstack"
)

type state struct {
	ClusterName      string
	Description      string
	InitialNodeCount int64

	AcsSettings
	ClusterInfo types.ClusterInfo
}

// AcsSettings - Basic configs to Cloudstack
type AcsSettings struct {
	SSHKeyPair      string
	ProjectID       string
	UsePrivateIP    bool
	ServiceOffering string
	TemplateID      string
	ZoneID          string
	NetworkID       string
	UserData        string

	Config
}

// Config - Cloudstack Configurations
type Config struct {
	EndPoint string
	Access   string
	Secret   string
}

// CloudstackConn - Connection to cloudstack
type CloudstackConn struct {
	cloudstackConnection *cloudstack.CloudStackClient
}

type project struct{}

// ACSDriver - Initiate Capabilities
type ACSDriver struct {
	driverCapabilities types.Capabilities
}

// NewDriver implement a driver to connect
func NewDriver() types.Driver {
	driver := &ACSDriver{
		driverCapabilities: types.Capabilities{
			Capabilities: make(map[int64]bool),
		},
	}

	driver.driverCapabilities.AddCapability(types.GetVersionCapability)
	driver.driverCapabilities.AddCapability(types.SetVersionCapability)
	driver.driverCapabilities.AddCapability(types.GetClusterSizeCapability)
	driver.driverCapabilities.AddCapability(types.SetClusterSizeCapability)

	return driver
}

// GetDriverCreateOptions Get Parameter in rancher UI.
func (d *ACSDriver) GetDriverCreateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["cluster-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Cluster name to be displayed in Rancher UI",
	}
	driverFlag.Options["cloudstack-endpoint"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Define Cloudstack API endpoint",
	}
	driverFlag.Options["cloudstack-access"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Access Key to authenticate in cloudstack",
	}
	driverFlag.Options["cloudstack-secret"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Secret Key to authenticate in cloudstack",
	}
	driverFlag.Options["description"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Description to project",
	}

	return &driverFlag, nil
}

// GetDriverUpdateOptions Default options to be used
func (d *ACSDriver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["node-count"] = &types.Flag{
		Type:  types.IntType,
		Usage: "Number of nodes",
	}
	return &driverFlag, nil
}

// Create effective create infra in cloudstack
func (d *ACSDriver) Create(ctx context.Context, opts *types.DriverOptions, _ *types.ClusterInfo) (*types.ClusterInfo, error) {
	state, err := getStateFromOpts(opts)
	if err != nil {
		return nil, err
	}

	info := &types.ClusterInfo{}

	/*
		Create two Projects in Cloudstack:
		First: Contains ETCD and Master elements, this project have a minimum 3 networks to HA,
		and all networks have full communications between them. VMs launched here need
		be born with essentials elements to be manager by operation teams like monitoring
		and autoscaling tools. The minimum number of nodes is in the code, 3 to each role
		(etcd and master).

		Second: In this project, worker nodes will be launched and user have the responsability
		to manager them. The initial number of this nodes are geted by "node-count" flag, configured
		by the user in rancher-ui.

		** IMPORTANT: This step its hard to be implement in Cloudstack from globo.com,
		they have particulary fields in project launch, not supported by default go-cloudstack lib.

		_, err = state.AcsSettings.createProject(state)
		if err != nil {
			return info, err
		}

	*/

	// Create instance in Cloudstack
	_, err = state.AcsSettings.createInstance(state)
	if err != nil {
		return info, err
	}

	return info, nil
}

func getStateFromOpts(driverOptions *types.DriverOptions) (state, error) {

	d := state{
		ClusterInfo: types.ClusterInfo{
			Metadata: map[string]string{},
		},
	}

	d.ClusterName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "cluster-name", "ClusterName").(string)
	d.Description = options.GetValueFromDriverOptions(driverOptions, types.StringType, "description", "Description").(string)
	d.AcsSettings.EndPoint = options.GetValueFromDriverOptions(driverOptions, types.StringType, "cloudstack-endpoint", "CloudstackEndPoint").(string)
	d.AcsSettings.Access = options.GetValueFromDriverOptions(driverOptions, types.StringType, "cloudstack-access").(string)
	d.AcsSettings.Secret = options.GetValueFromDriverOptions(driverOptions, types.StringType, "cloudstack-secret").(string)
	d.InitialNodeCount = options.GetValueFromDriverOptions(driverOptions, types.IntType, "node-count", "InitialNodeCount").(int64)

	return d, nil
}

func (c *AcsSettings) createProject(s state) (*cloudstack.CreateProjectResponse, error) {
	descriptionText := "aa"
	displayName := s.ClusterName

	cs, err := c.acsConn()
	if err != nil {
		//fmt.Errorf("Failed to connect")
		return nil, err
	}

	project := cs.Project.NewCreateProjectParams(descriptionText, displayName)

	p, err := cs.Project.CreateProject(project)
	if err != nil {
		//fmt.Errorf("Failed creating project %s", err)
		return nil, err
	}

	return p, nil
}

func (c *AcsSettings) createInstance(s state) (*cloudstack.DeployVirtualMachineResponse, error) {

	cs, err := c.acsConn()
	if err != nil {
		//fmt.Errorf("Failed to connect to cloudstack")
		return nil, err
	}

	p := cs.VirtualMachine.NewDeployVirtualMachineParams(c.ServiceOffering, c.TemplateID, c.ZoneID)

	p.SetDisplayname(s.ClusterName)

	//TODO: Get auto attributes
	p.SetName(s.ClusterName)
	p.SetNetworkids([]string{c.NetworkID})
	p.SetProjectid(c.ProjectID)
	p.SetServiceofferingid(c.ServiceOffering)
	p.SetTemplateid(c.TemplateID)
	p.SetZoneid(c.ZoneID)

	vm, err := cs.VirtualMachine.DeployVirtualMachine(p)
	if err != nil {
		fmt.Printf("Error creating the new instance: %s\n", err)
	} else {
		fmt.Printf("Success create instances")
	}

	return vm, nil
}

func (c *AcsSettings) acsConn() (*cloudstack.CloudStackClient, error) {
	cloudstackConnection := cloudstack.NewClient(c.EndPoint, c.Access, c.Secret, false)
	return cloudstackConnection, nil
}

// Update Will be update infra
func (d *ACSDriver) Update(ctx context.Context, info *types.ClusterInfo, opts *types.DriverOptions) (*types.ClusterInfo, error) {
	return info, nil
}

// PostCheck confirm settings after create
func (d *ACSDriver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	return info, nil
}

// Remove delete provider cluster
func (d *ACSDriver) Remove(ctx context.Context, info *types.ClusterInfo) error {
	return fmt.Errorf("Not implemented")
}

// GetVersion - dã
func (d *ACSDriver) GetVersion(ctx context.Context, info *types.ClusterInfo) (*types.KubernetesVersion, error) {
	k8s := &types.KubernetesVersion{}
	return k8s, nil
}

// SetVersion define version to be used
func (d *ACSDriver) SetVersion(ctx context.Context, info *types.ClusterInfo, version *types.KubernetesVersion) error {
	return nil
}

// GetClusterSize Consulting size of the cluster
func (d *ACSDriver) GetClusterSize(ctx context.Context, info *types.ClusterInfo) (*types.NodeCount, error) {
	count := &types.NodeCount{}
	return count, nil
}

// SetClusterSize setup the cluster size
func (d *ACSDriver) SetClusterSize(ctx context.Context, info *types.ClusterInfo, count *types.NodeCount) error {
	return fmt.Errorf("Not implemented")
}

// GetCapabilities Get information about k8s
func (d *ACSDriver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	cap := &types.Capabilities{}
	return cap, nil
}

// RemoveLegacyServiceAccount Init cleanup
func (d *ACSDriver) RemoveLegacyServiceAccount(ctx context.Context, info *types.ClusterInfo) error {
	return nil
}

// ETCDSave generate backup in ETCD cluster
func (d *ACSDriver) ETCDSave(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return fmt.Errorf("Not implemented")
}

// ETCDRestore Implement restore options
func (d *ACSDriver) ETCDRestore(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return nil
}

// ETCDRemoveSnapshot Remove snapshot
func (d *ACSDriver) ETCDRemoveSnapshot(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return fmt.Errorf("Not implemented")
}

// GetK8SCapabilities check cluster options
func (d *ACSDriver) GetK8SCapabilities(ctx context.Context, options *types.DriverOptions) (*types.K8SCapabilities, error) {
	k8s := &types.K8SCapabilities{}
	return k8s, nil
}
