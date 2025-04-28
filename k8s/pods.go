package k8s

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetPodsByPrefix returns pods that match the given name prefix
func GetPodsByPrefix(clientset *kubernetes.Clientset, prefix string) ([]map[string]interface{}, error) {
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var podList []map[string]interface{}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, prefix) {
			podInfo := map[string]interface{}{
				"name":       pod.Name,
				"namespace":  pod.Namespace,
				"containers": []string{},
				"status":     string(pod.Status.Phase),
			}
			for _, container := range pod.Spec.Containers {
				podInfo["containers"] = append(podInfo["containers"].([]string), container.Name)
			}
			podList = append(podList, podInfo)
		}
	}
	return podList, nil
} 