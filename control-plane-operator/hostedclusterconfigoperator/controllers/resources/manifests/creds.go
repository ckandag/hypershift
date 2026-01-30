package manifests

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AWSIngressCloudCredsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-ingress-operator",
			Name:      "cloud-credentials",
		},
	}
}

func AWSImageRegistryCloudCredsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-image-registry",
			Name:      "installer-cloud-credentials",
		},
	}
}

func AWSStorageCloudCredsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-cluster-csi-drivers",
			Name:      "ebs-cloud-credentials",
		},
	}
}

// Azure credential secrets for hosted cluster operators

func AzureIngressCloudCredsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-ingress-operator",
			Name:      "cloud-credentials",
		},
	}
}

func AzureImageRegistryCloudCredsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-image-registry",
			Name:      "installer-cloud-credentials",
		},
	}
}

func AzureDiskCSICloudCredsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-cluster-csi-drivers",
			Name:      "azure-disk-credentials",
		},
	}
}

func AzureFileCSICloudCredsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-cluster-csi-drivers",
			Name:      "azure-file-credentials",
		},
	}
}

// GCP credential secrets for hosted cluster operators

// GCPPDStorageCloudCredsSecret returns the secret for the GCP PD CSI driver
// in the guest cluster. This secret contains WIF credentials for the CSI
// node daemonset to authenticate with GCP for volume operations.
func GCPPDStorageCloudCredsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-cluster-csi-drivers",
			Name:      "gcp-pd-cloud-credentials",
		},
	}
}
