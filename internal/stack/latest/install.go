package latest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kplane-dev/kplane/internal/assets"
	"github.com/kplane-dev/kplane/internal/kubectl"
)

type Images struct {
	Apiserver string
	Operator  string
	Etcd      string
}

type InstallOptions struct {
	Context     string
	Namespace   string
	Images      Images
	CRDSource   string
	InstallCRDs bool
	Logf        func(format string, args ...any)
}

func Install(ctx context.Context, opts InstallOptions) error {
	logf(opts, "creating namespace %s", opts.Namespace)
	if err := kubectl.CreateNamespace(ctx, opts.Context, opts.Namespace); err != nil {
		return err
	}

	logf(opts, "generating certs and secrets")
	certs, err := generateCerts()
	if err != nil {
		return err
	}

	logf(opts, "applying secrets")
	if err := applySecrets(ctx, opts, certs); err != nil {
		return err
	}

	logf(opts, "deploying etcd")
	if err := applyEtcd(ctx, opts); err != nil {
		return err
	}

	logf(opts, "deploying apiserver")
	if err := applyApiserver(ctx, opts); err != nil {
		return err
	}

	logf(opts, "deploying ingress controller")
	if err := applyIngressController(ctx, opts); err != nil {
		return err
	}

	logf(opts, "configuring ingress route")
	if err := applyIngressRoute(ctx, opts); err != nil {
		return err
	}

	logf(opts, "applying operator config")
	if err := applyOperatorConfig(ctx, opts); err != nil {
		return err
	}

	logf(opts, "creating apiserver token auth secret")
	if err := applyTokenAuthSecret(ctx, opts, certs); err != nil {
		return err
	}

	logf(opts, "creating apiserver kubeconfig secret")
	if err := applyApiserverKubeconfig(ctx, opts, certs); err != nil {
		return err
	}

	if opts.InstallCRDs {
		logf(opts, "installing CRDs from %s", opts.CRDSource)
		if err := applyCRDs(ctx, opts); err != nil {
			return err
		}
	}

	logf(opts, "applying default ControlPlaneClass")
	if err := applyDefaultControlPlaneClass(ctx, opts); err != nil {
		return err
	}

	logf(opts, "deploying controlplane-operator")
	if err := applyOperator(ctx, opts); err != nil {
		return err
	}

	return nil
}

type certBundle struct {
	ServiceAccountKey    []byte
	ServiceAccountPub    []byte
	ClusterCAKey         []byte
	ClusterCACert        []byte
	KubeletClientKey     []byte
	KubeletClientCert    []byte
	ApiserverTLSKey      []byte
	ApiserverTLSCert     []byte
	ApiserverAdminKey    []byte
	ApiserverAdminCert   []byte
	ApiserverAdminToken  string
	ApiserverServerName  string
	ApiserverServiceAddr string
}

func generateCerts() (*certBundle, error) {
	now := time.Now()

	caKey, caKeyPEM, err := generateRSAKey()
	if err != nil {
		return nil, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "kplane-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caCertPEM, caCert, err := signCert(caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	saKey, saKeyPEM, err := generateRSAKey()
	if err != nil {
		return nil, err
	}
	saPubPEM, err := encodePublicKeyPEM(&saKey.PublicKey)
	if err != nil {
		return nil, err
	}

	kubeletKey, kubeletKeyPEM, err := generateRSAKey()
	if err != nil {
		return nil, err
	}
	kubeletTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   "system:kube-apiserver",
			Organization: []string{"system:masters"},
		},
		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.AddDate(1, 0, 0),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}
	kubeletCertPEM, _, err := signCert(kubeletTemplate, caCert, &kubeletKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	apiserverKey, apiserverKeyPEM, err := generateRSAKey()
	if err != nil {
		return nil, err
	}
	apiserverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "kplane-apiserver"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{
			"kplane-apiserver",
			"kplane-apiserver.kplane-system",
			"kplane-apiserver.kplane-system.svc",
			"kplane-apiserver.kplane-system.svc.cluster.local",
			"localhost",
			"*.kplane.example",
			"*.join.kplane.example",
		},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	apiserverCertPEM, _, err := signCert(apiserverTemplate, caCert, &apiserverKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	adminKey, adminKeyPEM, err := generateRSAKey()
	if err != nil {
		return nil, err
	}
	adminTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(4),
		Subject:      pkix.Name{CommonName: "kplane-admin"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	adminCertPEM, _, err := signCert(adminTemplate, caCert, &adminKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	adminToken, err := generateToken()
	if err != nil {
		return nil, err
	}

	return &certBundle{
		ServiceAccountKey:    saKeyPEM,
		ServiceAccountPub:    saPubPEM,
		ClusterCAKey:         caKeyPEM,
		ClusterCACert:        caCertPEM,
		KubeletClientKey:     kubeletKeyPEM,
		KubeletClientCert:    kubeletCertPEM,
		ApiserverTLSKey:      apiserverKeyPEM,
		ApiserverTLSCert:     apiserverCertPEM,
		ApiserverAdminKey:    adminKeyPEM,
		ApiserverAdminCert:   adminCertPEM,
		ApiserverAdminToken:  adminToken,
		ApiserverServerName:  "kplane-apiserver",
		ApiserverServiceAddr: "https://kplane-apiserver.kplane-system.svc.cluster.local:6443",
	}, nil
}

func applySecrets(ctx context.Context, opts InstallOptions, certs *certBundle) error {
	secretYaml := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: apiserver-serviceaccount-keys
  namespace: %s
type: Opaque
stringData:
  sa.key: |-
%s
  sa.pub: |-
%s
---
apiVersion: v1
kind: Secret
metadata:
  name: kplane-cluster-signing-keys
  namespace: %s
type: Opaque
stringData:
  ca.crt: |-
%s
  ca.key: |-
%s
---
apiVersion: v1
kind: Secret
metadata:
  name: kplane-kubelet-client
  namespace: %s
type: Opaque
stringData:
  client.crt: |-
%s
  client.key: |-
%s
---
apiVersion: v1
kind: Secret
metadata:
  name: kplane-apiserver-tls
  namespace: %s
type: kubernetes.io/tls
stringData:
  tls.crt: |-
%s
  tls.key: |-
%s
`, opts.Namespace,
		indentPEM(certs.ServiceAccountKey),
		indentPEM(certs.ServiceAccountPub),
		opts.Namespace,
		indentPEM(certs.ClusterCACert),
		indentPEM(certs.ClusterCAKey),
		opts.Namespace,
		indentPEM(certs.KubeletClientCert),
		indentPEM(certs.KubeletClientKey),
		opts.Namespace,
		indentPEM(certs.ApiserverTLSCert),
		indentPEM(certs.ApiserverTLSKey),
	)
	return kubectl.Apply(ctx, kubectl.ApplyOptions{Context: opts.Context, Stdin: []byte(secretYaml)})
}

func applyEtcd(ctx context.Context, opts InstallOptions) error {
	manifest := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: kplane-etcd
  namespace: %s
spec:
  selector:
    app: kplane-etcd
  ports:
    - name: client
      port: 2379
      targetPort: 2379
    - name: peer
      port: 2380
      targetPort: 2380
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kplane-etcd
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kplane-etcd
  template:
    metadata:
      labels:
        app: kplane-etcd
    spec:
      containers:
        - name: etcd
          image: %s
          env:
            - name: ALLOW_NONE_AUTHENTICATION
              value: "yes"
            - name: ETCD_LISTEN_CLIENT_URLS
              value: "http://0.0.0.0:2379"
            - name: ETCD_ADVERTISE_CLIENT_URLS
              value: "http://kplane-etcd.%s.svc.cluster.local:2379"
            - name: ETCD_LISTEN_PEER_URLS
              value: "http://0.0.0.0:2380"
            - name: ETCD_INITIAL_ADVERTISE_PEER_URLS
              value: "http://kplane-etcd.%s.svc.cluster.local:2380"
            - name: ETCD_INITIAL_CLUSTER
              value: "default=http://kplane-etcd.%s.svc.cluster.local:2380"
            - name: ETCD_NAME
              value: "default"
          ports:
            - containerPort: 2379
            - containerPort: 2380
`, opts.Namespace, opts.Namespace, opts.Images.Etcd, opts.Namespace, opts.Namespace, opts.Namespace)
	return kubectl.Apply(ctx, kubectl.ApplyOptions{Context: opts.Context, Stdin: []byte(manifest)})
}

func applyApiserver(ctx context.Context, opts InstallOptions) error {
	manifest := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: kplane-apiserver
  namespace: %s
spec:
  selector:
    app: kplane-apiserver
  ports:
    - name: https
      port: 6443
      targetPort: 6443
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kplane-apiserver
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kplane-apiserver
  template:
    metadata:
      labels:
        app: kplane-apiserver
    spec:
      containers:
        - name: apiserver
          image: %s
          imagePullPolicy: IfNotPresent
          args:
            - --etcd-servers=http://kplane-etcd.%s.svc.cluster.local:2379
            - --secure-port=6443
            - --service-cluster-ip-range=10.96.0.0/12
            - --allow-privileged=true
            - --authorization-mode=AlwaysAllow
            - --anonymous-auth=true
            - --enable-bootstrap-token-auth=true
            - --api-audiences=https://kplane.local
            - --service-account-issuer=https://kplane.local
            - --service-account-signing-key-file=/var/run/kplane/sa/sa.key
            - --service-account-key-file=/var/run/kplane/sa/sa.pub
            - --service-account-lookup=false
            - --token-auth-file=/var/run/kplane/token/token.csv
            - --kubelet-client-certificate=/var/run/kplane/kubelet-client/client.crt
            - --kubelet-client-key=/var/run/kplane/kubelet-client/client.key
            - --kubelet-certificate-authority=/var/run/kplane/cluster-signing/ca.crt
            - --tls-cert-file=/var/run/kplane/tls/tls.crt
            - --tls-private-key-file=/var/run/kplane/tls/tls.key
            - --client-ca-file=/var/run/kplane/cluster-signing/ca.crt
            - --v=2
          ports:
            - containerPort: 6443
          volumeMounts:
            - name: apiserver-tls
              mountPath: /var/run/kplane/tls
            - name: sa-keys
              mountPath: /var/run/kplane/sa
              readOnly: true
            - name: cluster-signing-keys
              mountPath: /var/run/kplane/cluster-signing
              readOnly: true
            - name: kubelet-client
              mountPath: /var/run/kplane/kubelet-client
              readOnly: true
            - name: token-auth
              mountPath: /var/run/kplane/token
              readOnly: true
      volumes:
        - name: apiserver-tls
          secret:
            secretName: kplane-apiserver-tls
        - name: sa-keys
          secret:
            secretName: apiserver-serviceaccount-keys
        - name: cluster-signing-keys
          secret:
            secretName: kplane-cluster-signing-keys
        - name: kubelet-client
          secret:
            secretName: kplane-kubelet-client
        - name: token-auth
          secret:
            secretName: apiserver-token-auth
`, opts.Namespace, opts.Namespace, opts.Images.Apiserver, opts.Namespace)
	return kubectl.Apply(ctx, kubectl.ApplyOptions{Context: opts.Context, Stdin: []byte(manifest)})
}

func applyOperatorConfig(ctx context.Context, opts InstallOptions) error {
	raw, err := assets.ControlplaneOperator.ReadFile("controlplane-operator/config/operatorconfig.yaml")
	if err != nil {
		return fmt.Errorf("read operator config: %w", err)
	}
	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: operator-config
  namespace: %s
data:
  operatorconfig.yaml: |-
%s
`, opts.Namespace, indentLiteral(string(raw)))
	return kubectl.Apply(ctx, kubectl.ApplyOptions{Context: opts.Context, Stdin: []byte(manifest)})
}

func applyApiserverKubeconfig(ctx context.Context, opts InstallOptions, certs *certBundle) error {
	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: kplane-apiserver
  cluster:
    server: %s
    insecure-skip-tls-verify: true
users:
- name: kplane-admin
  user:
    token: %s
contexts:
- name: kplane-apiserver
  context:
    cluster: kplane-apiserver
    user: kplane-admin
current-context: kplane-apiserver
`, certs.ApiserverServiceAddr, certs.ApiserverAdminToken)

	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: apiserver-kubeconfig
  namespace: %s
type: Opaque
stringData:
  kubeconfig: |-
%s
`, opts.Namespace, indentLiteral(kubeconfig))

	return kubectl.Apply(ctx, kubectl.ApplyOptions{Context: opts.Context, Stdin: []byte(manifest)})
}

func applyTokenAuthSecret(ctx context.Context, opts InstallOptions, certs *certBundle) error {
	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: apiserver-token-auth
  namespace: %s
type: Opaque
stringData:
  token.csv: "%s,kplane-admin,1,system:masters"
`, opts.Namespace, certs.ApiserverAdminToken)
	return kubectl.Apply(ctx, kubectl.ApplyOptions{Context: opts.Context, Stdin: []byte(manifest)})
}

func applyCRDs(ctx context.Context, opts InstallOptions) error {
	if opts.CRDSource == "" {
		return fmt.Errorf("crd source is required when install-crds is true")
	}
	return kubectl.ApplyKustomize(ctx, opts.Context, opts.CRDSource)
}

func applyDefaultControlPlaneClass(ctx context.Context, opts InstallOptions) error {
	classYaml := `apiVersion: controlplane.kplane.dev/v1alpha1
kind: ControlPlaneClass
metadata:
  name: starter
spec:
  addons:
    - starter
  auth:
    model: basic
    defaultRole: admin
  modesAllowed:
    - Virtual
`
	return kubectl.Apply(ctx, kubectl.ApplyOptions{Context: opts.Context, Stdin: []byte(classYaml)})
}

func applyOperator(ctx context.Context, opts InstallOptions) error {
	tempDir, err := os.MkdirTemp("", "kplane-operator-kustomize-*")
	if err != nil {
		return fmt.Errorf("create temp kustomize dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if _, err := fs.Stat(assets.ControlplaneOperator, "controlplane-operator/config/default/kustomization.yaml"); err != nil {
		return fmt.Errorf("embedded operator assets missing; rebuild kplane binary: %w", err)
	}

	if err := writeEmbeddedDir(assets.ControlplaneOperator, "controlplane-operator", tempDir); err != nil {
		return err
	}

	defaultKustomization := filepath.Join(tempDir, "controlplane-operator", "config", "default", "kustomization.yaml")
	if _, err := os.Stat(defaultKustomization); err != nil {
		return fmt.Errorf("operator assets not found in temp dir; rebuild kplane binary: %w", err)
	}
	if err := updateDefaultNamespace(defaultKustomization, opts.Namespace); err != nil {
		return err
	}

	managerKustomization := filepath.Join(tempDir, "controlplane-operator", "config", "manager", "kustomization.yaml")
	if _, err := os.Stat(managerKustomization); err != nil {
		return fmt.Errorf("operator assets not found in temp dir; rebuild kplane binary: %w", err)
	}
	if err := updateManagerImage(managerKustomization, opts.Images.Operator); err != nil {
		return err
	}

	applyPath := filepath.Join(tempDir, "controlplane-operator", "config", "default")
	return kubectl.ApplyKustomize(ctx, opts.Context, applyPath)
}

func applyIngressController(ctx context.Context, opts InstallOptions) error {
	const url = "https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.11.3/deploy/static/provider/kind/deploy.yaml"
	if err := kubectl.ApplyURL(ctx, opts.Context, url); err != nil {
		return err
	}
	return kubectl.RolloutStatus(ctx, opts.Context, "ingress-nginx", "deployment", "ingress-nginx-controller", 3*time.Minute)
}

func applyIngressRoute(ctx context.Context, opts InstallOptions) error {
	manifest := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kplane-apiserver
  namespace: %s
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: "HTTPS"
    nginx.ingress.kubernetes.io/proxy-ssl-verify: "off"
spec:
  ingressClassName: nginx
  rules:
    - http:
        paths:
          - path: /clusters/
            pathType: Prefix
            backend:
              service:
                name: kplane-apiserver
                port:
                  number: 6443
`, opts.Namespace)
	return kubectl.Apply(ctx, kubectl.ApplyOptions{Context: opts.Context, Stdin: []byte(manifest)})
}

func generateRSAKey() (*rsa.PrivateKey, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate rsa key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return key, keyPEM, nil
}

func encodePublicKeyPEM(key *rsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("encode public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

func signCert(template, parent *x509.Certificate, pub *rsa.PublicKey, parentKey *rsa.PrivateKey) ([]byte, *x509.Certificate, error) {
	der, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentKey)
	if err != nil {
		return nil, nil, fmt.Errorf("sign cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse cert: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), cert, nil
}

func indentPEM(b []byte) string {
	return indentLiteral(string(b))
}

func indentLiteral(value string) string {
	value = strings.TrimSuffix(value, "\n")
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}

func imageName(image string) string {
	parts := strings.Split(image, ":")
	return parts[0]
}

func imageTag(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return "latest"
	}
	return parts[len(parts)-1]
}

func writeEmbeddedDir(source fs.FS, src, dst string) error {
	return fs.WalkDir(source, src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func updateDefaultNamespace(path, namespace string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read kustomization: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "namespace:") {
			lines[i] = fmt.Sprintf("namespace: %s", namespace)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func updateManagerImage(path, image string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read manager kustomization: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	nameLine := fmt.Sprintf("  newName: %s", imageName(image))
	tagLine := fmt.Sprintf("  newTag: %s", imageTag(image))
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "newName:") {
			lines[i] = nameLine
		}
		if strings.HasPrefix(strings.TrimSpace(line), "newTag:") {
			lines[i] = tagLine
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func logf(opts InstallOptions, format string, args ...any) {
	if opts.Logf != nil {
		opts.Logf(format, args...)
	}
}

func encodeBase64(data []byte) string {
	return strings.TrimRight(base64.StdEncoding.EncodeToString(data), "\n")
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
