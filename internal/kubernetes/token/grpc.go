/*
Copyright 2024 The Kubernetes-CSI-Addons Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package token

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const bearerPrefix = "Bearer "
const authorizationKey = "authorization"

func WithServiceAccountToken() grpc.DialOption {
	return grpc.WithUnaryInterceptor(addAuthorizationHeader)
}

func addAuthorizationHeader(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	token, err := getToken()
	if err != nil {
		return err
	}

	authCtx := metadata.AppendToOutgoingContext(ctx, authorizationKey, bearerPrefix+token)
	return invoker(authCtx, method, req, reply, cc, opts...)
}

func getToken() (string, error) {
	return readFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
}

func AuthorizationInterceptor(kubeclient kubernetes.Clientset) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := authorizeConnection(ctx, kubeclient); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func authorizeConnection(ctx context.Context, kubeclient kubernetes.Clientset) error {

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader, ok := md[authorizationKey]
	if !ok || len(authHeader) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization token")
	}

	token := authHeader[0]
	isValidated, err := validateBearerToken(ctx, token, kubeclient)
	if err != nil {
		return err
	}
	if !isValidated {

		return status.Errorf(codes.Unauthenticated, "invalid token")
	}
	return nil
}

func parseToken(authHeader string) string {
	return strings.TrimPrefix(authHeader, bearerPrefix)
}

func validateBearerToken(ctx context.Context, token string, kubeclient kubernetes.Clientset) (bool, error) {
	tokenReview := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: parseToken(token),
		},
	}
	result, err := kubeclient.AuthenticationV1().TokenReviews().Create(ctx, tokenReview, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to review token %w", err)
	}
	if result.Status.Authenticated {
		return true, nil
	}
	return false, nil
}

func readFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		err = file.Close()
		if err != nil {
			log.Printf("failed to close file %q: %v", filePath, err)
		}
	}()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GenerateSelfSignedCert generates a self-signed certificate and key for use in a TLS connection.
func GenerateSelfSignedCert() (tls.Certificate, error) {
	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"k8s-addons-sidecar-server"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		IsCA: true,
	}

	// Self-sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Encode certificate and key into PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	// Load the certificate into a tls.Certificate
	return tls.X509KeyPair(certPEM, keyPEM)
}
