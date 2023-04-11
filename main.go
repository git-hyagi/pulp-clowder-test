package main

import (
	"context"
	"encoding/json"
	"fmt"

	pulp "github.com/pulp/pulp-operator/api/v1alpha1"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

const pulpAPIVersion = "repo-manager.pulpproject.org/v1alpha1"

func main() {

	const namespace = "pulp"
	const externalDBSecretName = "external-database"
	const externalRedisSecretName = "external-redis"
	const s3SecretName = "test-s3"
	const crName = "example-pulp"

	ctx := context.TODO()

	config := ctrl.GetConfigOrDie()
	clientset := kubernetes.NewForConfigOrDie(config)

	// example of DB secret from clowder
	// I think this can be gathered from clowder.LoadedConfig.RdsCa()
	databaseSecretFromClowder := clowder.DatabaseConfig{
		AdminPassword: "pass",
		AdminUsername: "user",
		Hostname:      "dbhost",
		Name:          "dbname",
		Password:      "dbpass",
		Port:          5432,
		SslMode:       "disable",
		Username:      "dbuser",
	}

	if _, err := createDBSecret(ctx, clientset, databaseSecretFromClowder, externalDBSecretName, namespace); err != nil {
		fmt.Println(err)
	}

	// example of Redis secret from clowder
	redisSecretFromClowder := clowder.InMemoryDBConfig{
		Hostname: "example.redis.local",
		Port:     6379,
	}

	if _, err := createRedisSecret(ctx, clientset, redisSecretFromClowder, externalRedisSecretName, namespace); err != nil {
		fmt.Println(err)
	}

	// example of objectStorage from clowder
	accessKey := "test"
	secretKey := "test"
	objStorageFromClowder := clowder.ObjectStoreConfig{
		Hostname:  "endpoint",
		Port:      9292,
		AccessKey: &accessKey,
		SecretKey: &secretKey,
		Tls:       false,
		Buckets: []clowder.ObjectStoreBucket{
			{
				AccessKey:     &accessKey,
				SecretKey:     &secretKey,
				RequestedName: "reqname",
				Name:          "pulp",
			},
		},
	}

	if _, err := createObjStorage(ctx, clientset, objStorageFromClowder, s3SecretName, namespace); err != nil {
		fmt.Println(err)
	}

	// sample CR
	pulp := &pulp.Pulp{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pulp",
			APIVersion: pulpAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: namespace,
		},
		Spec: pulp.PulpSpec{
			Database: pulp.Database{
				ExternalDBSecret: externalDBSecretName,
			},
			Cache: pulp.Cache{
				ExternalCacheSecret: externalRedisSecretName,
			},
			ObjectStorageS3Secret: s3SecretName,
			PulpSettings: runtime.RawExtension{
				Raw: []byte(`{"aws_s3_endpoint_url": "http://` + objStorageFromClowder.Hostname + `"}`),
			},
		},
	}
	if body, err := createSampleCR(ctx, clientset, pulp, namespace, crName); err != nil {
		fmt.Println("err: ", err)
		fmt.Println("body: ", string(body))
	}
}

// createDBSecret creates the secret to use an external PostgreSQL
func createDBSecret(ctx context.Context, clientSet *kubernetes.Clientset, clowderSecret clowder.DatabaseConfig, secretName, namespace string) (*corev1.Secret, error) {

	// convert into expected operator format
	secretData := map[string]string{
		"POSTGRES_HOST":     clowderSecret.Hostname,
		"POSTGRES_PORT":     fmt.Sprint(clowderSecret.Port),
		"POSTGRES_USERNAME": clowderSecret.Username,
		"POSTGRES_PASSWORD": clowderSecret.Password,
		"POSTGRES_DB_NAME":  "pulp",
		"POSTGRES_SSLMODE":  clowderSecret.SslMode,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		StringData: secretData,
	}

	return clientSet.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
}

// createRedisSecret creates the secret to use an external Redis instance
func createRedisSecret(ctx context.Context, clientSet *kubernetes.Clientset, clowderSecret clowder.InMemoryDBConfig, secretName, namespace string) (*corev1.Secret, error) {

	password := ""
	if clowderSecret.Password != nil {
		password = *clowderSecret.Password
	}

	// convert into expected operator format
	secretData := map[string]string{
		"REDIS_HOST":     clowderSecret.Hostname,
		"REDIS_PORT":     fmt.Sprint(clowderSecret.Port),
		"REDIS_PASSWORD": password,
		"REDIS_DB":       "",
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		StringData: secretData,
	}

	return clientSet.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
}

// createObjStorage creates the secret to use s3
func createObjStorage(ctx context.Context, clientSet *kubernetes.Clientset, clowderSecret clowder.ObjectStoreConfig, secretName, namespace string) (*corev1.Secret, error) {

	accessKey := ""
	secretKey := ""
	region := "us-east-1"
	if clowderSecret.AccessKey != nil {
		accessKey = *clowderSecret.AccessKey
	}
	if clowderSecret.SecretKey != nil {
		secretKey = *clowderSecret.SecretKey
	}
	if clowderSecret.Buckets[0].Region != nil {
		secretKey = *clowderSecret.Buckets[0].Region
	}

	// convert into expected operator format
	secretData := map[string]string{
		"s3-access-key-id":     accessKey,
		"s3-secret-access-key": secretKey,
		"s3-bucket-name":       clowderSecret.Buckets[0].Name,
		"s3-region":            region,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		StringData: secretData,
	}

	return clientSet.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
}

// createSampleCR provisions Pulp CR
func createSampleCR(ctx context.Context, clientSet *kubernetes.Clientset, pulp *pulp.Pulp, namespace, crName string) ([]byte, error) {

	body, _ := json.Marshal(pulp)
	return clientSet.
		RESTClient().
		Post().
		AbsPath("/apis/" + pulpAPIVersion).
		Namespace(namespace).
		Resource("pulps").
		Name(crName).
		Body(body).
		DoRaw(ctx)
}
