package controller

import (
	"bytes"
	"context"
	"errors"
	"time"

	n "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/glog"
)

func (c AppGwIngressController) configHasChanged(ctx context.Context, appGw *n.ApplicationGateway) bool {
	jsonConfig, err := appGw.MarshalJSON()
	if err != nil {
		glog.Error("Could not marshal App Gwy to compare w/ cache; Will not use cache.", err)
		return true
	}
	// The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
	return bytes.Compare(*c.appGwConfigCache, jsonConfig) != 0
}

func (c AppGwIngressController) deployConfig(ctx context.Context, appGw *n.ApplicationGateway) error {
	glog.V(1).Infoln("START Application Gateway configuration")
	defer glog.V(1).Infoln("FINISH Application Gateway configuration")

	deploymentStart := time.Now()

	// Keep a reference to the current version of the config
	if jsonConfig, err := appGw.MarshalJSON(); err != nil {
		glog.Error("Could not marshal App Gwy config for caching; Will not use cache.", err)
	} else {
		// Ensure we have an empty slice
		// TODO(draychev): use to.ByteSlicePtr() once it is merged
		byteSlice := make([]byte, len(jsonConfig))
		*c.appGwConfigCache = byteSlice
		copy(*c.appGwConfigCache, jsonConfig)
	}

	// Initiate deployment
	appGwFuture, err := c.appGwClient.CreateOrUpdate(ctx, c.appGwIdentifier.ResourceGroup, c.appGwIdentifier.AppGwName, *appGw)

	if err != nil {
		glog.Warningln("App Gwy configuration request failed:", err)
		return errors.New("unable to send CreateOrUpdate request")
	}

	// Wait until deployment completes
	err = appGwFuture.WaitForCompletionRef(ctx, c.appGwClient.BaseClient.Client)
	glog.V(1).Infof("Deployment of configuration to App Gwy took %+v", time.Now().Sub(deploymentStart).String())

	if err != nil {
		errorMessage := "Application Gateway configuration failed."
		glog.Warning(errorMessage, err)
		return errors.New(errorMessage)
	}
	return nil
}

// addTags will add certain tags to Application Gateway
func addTags(appGw *n.ApplicationGateway) {
	if appGw.Tags == nil {
		appGw.Tags = make(map[string]*string)
	}
	// Identify the App Gateway as being exclusively managed by a Kubernetes Ingress.
	appGw.Tags[isManagedByK8sIngress] = to.StringPtr("true")
}
