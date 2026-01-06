// Package testutil provides testing utilities and mocks for NOVA tests.
package testutil

import (
	"context"

	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
)

// MockKubectlClient is a mock implementation of kubectl client for testing.
type MockKubectlClient struct {
	mock.Mock
}

func (m *MockKubectlClient) ApplyYAML(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockKubectlClient) ApplyYAMLContent(ctx context.Context, content string) error {
	args := m.Called(ctx, content)
	return args.Error(0)
}

func (m *MockKubectlClient) DeletePod(ctx context.Context, namespace, name string) error {
	args := m.Called(ctx, namespace, name)
	return args.Error(0)
}

func (m *MockKubectlClient) WaitForPodReady(ctx context.Context, namespace, name string, timeoutSeconds int) error {
	args := m.Called(ctx, namespace, name, timeoutSeconds)
	return args.Error(0)
}

func (m *MockKubectlClient) WaitForDeploymentReady(ctx context.Context, namespace, name string, timeoutSeconds int) error {
	args := m.Called(ctx, namespace, name, timeoutSeconds)
	return args.Error(0)
}

func (m *MockKubectlClient) WaitForStatefulSetReady(ctx context.Context, namespace, name string, timeoutSeconds int) error {
	args := m.Called(ctx, namespace, name, timeoutSeconds)
	return args.Error(0)
}

func (m *MockKubectlClient) SecretExists(ctx context.Context, namespace, name string) bool {
	args := m.Called(ctx, namespace, name)
	return args.Bool(0)
}

func (m *MockKubectlClient) GetSecretData(ctx context.Context, namespace, name string) (map[string]string, error) {
	args := m.Called(ctx, namespace, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockKubectlClient) CreateSecret(ctx context.Context, namespace, name string, data map[string]string) error {
	args := m.Called(ctx, namespace, name, data)
	return args.Error(0)
}

func (m *MockKubectlClient) LabelNamespace(ctx context.Context, namespace, key, value string) error {
	args := m.Called(ctx, namespace, key, value)
	return args.Error(0)
}

func (m *MockKubectlClient) GetNodes(ctx context.Context) ([]corev1.Node, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]corev1.Node), args.Error(1)
}

func (m *MockKubectlClient) LabelNode(ctx context.Context, nodeName, key, value string) error {
	args := m.Called(ctx, nodeName, key, value)
	return args.Error(0)
}

func (m *MockKubectlClient) TaintNode(ctx context.Context, nodeName, key, value, effect string) error {
	args := m.Called(ctx, nodeName, key, value, effect)
	return args.Error(0)
}

func (m *MockKubectlClient) UntaintNode(ctx context.Context, nodeName, key string) error {
	args := m.Called(ctx, nodeName, key)
	return args.Error(0)
}

// MockHelmClient is a mock implementation of Helm client for testing.
type MockHelmClient struct {
	mock.Mock
}

func (m *MockHelmClient) AddRepository(ctx context.Context, name, url string) error {
	args := m.Called(ctx, name, url)
	return args.Error(0)
}

func (m *MockHelmClient) UpdateRepositories(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockHelmClient) Install(ctx context.Context, releaseName, chart, namespace string, values map[string]interface{}, wait bool, timeout int) error {
	args := m.Called(ctx, releaseName, chart, namespace, values, wait, timeout)
	return args.Error(0)
}

func (m *MockHelmClient) Upgrade(ctx context.Context, releaseName, chart, namespace string, values map[string]interface{}, wait bool, timeout int) error {
	args := m.Called(ctx, releaseName, chart, namespace, values, wait, timeout)
	return args.Error(0)
}

func (m *MockHelmClient) ReleaseExists(ctx context.Context, releaseName, namespace string) (bool, error) {
	args := m.Called(ctx, releaseName, namespace)
	return args.Bool(0), args.Error(1)
}

func (m *MockHelmClient) Uninstall(ctx context.Context, releaseName, namespace string) error {
	args := m.Called(ctx, releaseName, namespace)
	return args.Error(0)
}

// MockMinikubeClient is a mock implementation of Minikube client for testing.
type MockMinikubeClient struct {
	mock.Mock
}

func (m *MockMinikubeClient) Start(ctx context.Context, profile string, nodes int, cpus int, memory string, driver string) error {
	args := m.Called(ctx, profile, nodes, cpus, memory, driver)
	return args.Error(0)
}

func (m *MockMinikubeClient) Stop(ctx context.Context, profile string) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

func (m *MockMinikubeClient) Delete(ctx context.Context, profile string) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

func (m *MockMinikubeClient) GetStatus(ctx context.Context, profile string) (string, error) {
	args := m.Called(ctx, profile)
	return args.String(0), args.Error(1)
}

func (m *MockMinikubeClient) GetNodeIP(ctx context.Context, profile, nodeName string) (string, error) {
	args := m.Called(ctx, profile, nodeName)
	return args.String(0), args.Error(1)
}

// MockDockerClient is a mock implementation of Docker client for testing.
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) Pull(ctx context.Context, image string) error {
	args := m.Called(ctx, image)
	return args.Error(0)
}

func (m *MockDockerClient) ImageExists(ctx context.Context, image string) (bool, error) {
	args := m.Called(ctx, image)
	return args.Bool(0), args.Error(1)
}

func (m *MockDockerClient) ContainerExists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockDockerClient) StartContainer(ctx context.Context, name string, image string, ports map[string]string, volumes map[string]string, env map[string]string) error {
	args := m.Called(ctx, name, image, ports, volumes, env)
	return args.Error(0)
}

func (m *MockDockerClient) StopContainer(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockDockerClient) RemoveContainer(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
