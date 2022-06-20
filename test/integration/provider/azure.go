package provider

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	pointer "k8s.io/utils/pointer"

	provider "github.com/gardener/machine-controller-manager-provider-azure/pkg/azure"
	"github.com/gardener/machine-controller-manager-provider-azure/pkg/spi"
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
)

// deleteDisk deletes the specified disk on Azure
func deleteDisk(clients spi.AzureDriverClientsInterface, resourceGroup, diskID string) error {
	ctx := context.TODO()
	diskDeleteFuture, err := clients.GetDisk().Delete(ctx, resourceGroup, diskID)
	if err != nil {
		fmt.Printf("Delete operation failed on disk %s with error %s,", diskID, err.Error())
		return err
	}

	if err = diskDeleteFuture.WaitForCompletionRef(ctx, clients.GetClient()); err != nil {
		fmt.Printf("Delete operation failed on disk %s with error %s,", diskID, err.Error())
		return err
	}
	fmt.Printf("Deleted an orphaned Disk %s,", diskID)
	return nil

}

// deleteNICs deletes the specified NICs on Azure
func deleteNICs(clients spi.AzureDriverClientsInterface, resourceGroup, networkInterfaceName string) error {
	ctx := context.TODO()
	nicDeleteFuture, err := clients.GetNic().Delete(ctx, resourceGroup, networkInterfaceName)
	if err != nil {
		fmt.Printf("Delete operation failed on NIC %s with error %s,", networkInterfaceName, err.Error())
		return err
	}
	if err = nicDeleteFuture.WaitForCompletionRef(ctx, clients.GetClient()); err != nil {
		fmt.Printf("Delete operation failed on NIC %s with error %s,", networkInterfaceName, err.Error())
		return err
	}
	fmt.Printf("Deleted an orphaned NIC %s,", networkInterfaceName)
	return nil
}

// deleteVM deletes the specified Virtual Machine on Azure
func deleteVM(clients spi.AzureDriverClientsInterface, resourceGroup, VMName string) error {

	ctx := context.TODO()
	virtualMachineFuture, err := clients.GetVM().Delete(ctx, resourceGroup, VMName, pointer.BoolPtr(false))
	if err != nil {
		fmt.Printf("Delete operation failed on VM %s with error %s,", VMName, err.Error())
		return err
	}

	if err = virtualMachineFuture.WaitForCompletionRef(ctx, clients.GetClient()); err != nil {
		fmt.Printf("Delete operation failed on VM %s with error %s,", VMName, err.Error())
		return err
	}

	fmt.Printf("Deleted an orphan VM %s,", VMName)
	return nil
}

// getAzureClients returns Azure clients.
func getAzureClients(secretData map[string][]byte) (spi.AzureDriverClientsInterface, error) {

	localDriver := provider.NewAzureDriver(&spi.PluginSPIImpl{})
	client, err := localDriver.SPI.Setup(&v1.Secret{Data: secretData})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// getMachines returns the list of names of the machine objects in the control cluster.
func getMachines(machineClass *v1alpha1.MachineClass, secretData map[string][]byte) ([]string, error) {
	var (
		machines []string
		SPI      spi.PluginSPIImpl
	)

	driverprovider := provider.NewAzureDriver(&SPI)
	machineList, err := driverprovider.ListMachines(context.TODO(), &driver.ListMachinesRequest{
		MachineClass: machineClass,
		Secret: &v1.Secret{
			Data: secretData,
		},
	})

	if err != nil {
		return nil, err
	} else if len(machineList.MachineList) != 0 {
		fmt.Printf("\nAvailable Machines: ")
		for _, machine := range machineList.MachineList {
			fmt.Printf("%s,", machine)
			machines = append(machines, machine)
		}
	}
	return machines, nil
}

// getOrphanedDisks returns the list of orphaned disks.
func getOrphanedDisks(clients spi.AzureDriverClientsInterface, resourceGroup string) ([]string, error) {

	var orphanedDisks []string

	disks, err := clients.GetDisk().ListByResourceGroup(context.TODO(), resourceGroup)
	if err != nil {
		return orphanedDisks, err
	}

	for _, disk := range disks.Values() {
		if value, ok := disk.Tags[ITResourceTagKey]; ok && *value == ITResourceTagValue {
			orphanedDisks = append(orphanedDisks, *disk.Name)
		}
	}

	return orphanedDisks, nil
}

// getOrphanedNICs returns the list of orphaned NICs.
func getOrphanedNICs(clients spi.AzureDriverClientsInterface, resourceGroup string) ([]string, error) {
	ctx := context.TODO()

	var orphanedNICs []string

	networkInterfaces, err := clients.GetNic().List(ctx, resourceGroup)
	if err != nil {
		fmt.Printf("List operation failed on NIC from resource group %s with error %s,", resourceGroup, err.Error())
		return nil, err
	}

	for _, networkInterface := range networkInterfaces.Values() {
		if value, ok := networkInterface.Tags[ITResourceTagKey]; ok && ITResourceTagValue == *value {
			orphanedNICs = append(orphanedNICs, *networkInterface.Name)
		}
	}
	return orphanedNICs, nil
}

// getOrphanedVMs returns the list of orphaned virtual machines.
func getOrphanedVMs(clients spi.AzureDriverClientsInterface, resourceGroup string) ([]string, error) {

	var orphanedVMs []string

	virtualMachines, err := clients.GetVM().List(context.TODO(), resourceGroup, "")
	if err != nil {
		return orphanedVMs, err
	}

	for _, virtualMachine := range virtualMachines.Values() {
		if value, ok := virtualMachine.Tags[ITResourceTagKey]; ok && *value == ITResourceTagValue {
			orphanedVMs = append(orphanedVMs, *virtualMachine.Name)
		}
	}

	return orphanedVMs, nil
}

func cleanUpOrphanedResources(orphanedVms []string, orphanedVolumes []string, orphanedNICs []string, clients spi.AzureDriverClientsInterface, resourceGroup string) (delErrOrphanedVms []string, delErrOrphanedVolumes []string, delErrOrphanedNICs []string) {
	for _, virtualMachineName := range orphanedVms {
		err := deleteVM(clients, resourceGroup, virtualMachineName)
		if err != nil {
			delErrOrphanedVms = append(delErrOrphanedVms, virtualMachineName)
		}
	}
	for _, volumeID := range orphanedVolumes {
		err := deleteDisk(clients, resourceGroup, volumeID)
		if err != nil {
			delErrOrphanedVolumes = append(delErrOrphanedVolumes, volumeID)
		}
	}
	for _, networkInterfaceName := range orphanedNICs {
		err := deleteNICs(clients, resourceGroup, networkInterfaceName)
		if err != nil {
			delErrOrphanedNICs = append(delErrOrphanedNICs, networkInterfaceName)
		}
	}
	return
}
