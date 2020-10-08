package database

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	crossplanemeta "github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/provider-alibaba/apis/database/v1alpha1"
	aliv1alpha1 "github.com/crossplane/provider-alibaba/apis/v1alpha1"
	"github.com/crossplane/provider-alibaba/pkg/clients/rds"
)

const testName = "test"

func TestConnector(t *testing.T) {
	errBoom := errors.New("boom")

	type fields struct {
		client       client.Client
		usage        resource.Tracker
		newRDSClient func(ctx context.Context, accessKeyID, accessKeySecret, region string) (rds.Client, error)
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   error
	}{
		"NotRDSInstance": {
			reason: "Should return an error if the supplied managed resource is not an RDSInstance",
			args: args{
				mg: nil,
			},
			want: errors.New(errNotRDSInstance),
		},
		"TrackProviderConfigUsageError": {
			reason: "Errors tracking a ProviderConfigUsage should be returned",
			fields: fields{
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return errBoom }),
			},
			args: args{
				mg: &v1alpha1.RDSInstance{
					Spec: v1alpha1.RDSInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ProviderConfigReference: &runtimev1alpha1.Reference{},
						},
					},
				},
			},
			want: errors.Wrap(errBoom, errTrackUsage),
		},
		"GetProviderConfigError": {
			reason: "Errors getting a ProviderConfig should be returned",
			fields: fields{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			},
			args: args{
				mg: &v1alpha1.RDSInstance{
					Spec: v1alpha1.RDSInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ProviderConfigReference: &runtimev1alpha1.Reference{},
						},
					},
				},
			},
			want: errors.Wrap(errBoom, errGetProviderConfig),
		},
		"UnsupportedCredentialsError": {
			reason: "An error should be returned if the selected credentials source is unsupported",
			fields: fields{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
						t := obj.(*aliv1alpha1.ProviderConfig)
						*t = aliv1alpha1.ProviderConfig{
							Spec: aliv1alpha1.ProviderConfigSpec{
								ProviderConfigSpec: runtimev1alpha1.ProviderConfigSpec{
									Credentials: runtimev1alpha1.ProviderCredentials{
										Source: runtimev1alpha1.CredentialsSource("wat"),
									},
								},
							},
						}
						return nil
					}),
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			},
			args: args{
				mg: &v1alpha1.RDSInstance{
					Spec: v1alpha1.RDSInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ProviderConfigReference: &runtimev1alpha1.Reference{},
						},
					},
				},
			},
			want: errors.Errorf(errFmtUnsupportedCredSource, "wat"),
		},
		"GetProviderError": {
			reason: "Errors getting a Provider should be returned",
			fields: fields{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			},
			args: args{
				mg: &v1alpha1.RDSInstance{
					Spec: v1alpha1.RDSInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ProviderReference: &runtimev1alpha1.Reference{},
						},
					},
				},
			},
			want: errors.Wrap(errBoom, errGetProvider),
		},
		"NoConnectionSecretError": {
			reason: "An error should be returned if no connection secret was specified",
			fields: fields{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
						t := obj.(*aliv1alpha1.ProviderConfig)
						*t = aliv1alpha1.ProviderConfig{
							Spec: aliv1alpha1.ProviderConfigSpec{
								ProviderConfigSpec: runtimev1alpha1.ProviderConfigSpec{
									Credentials: runtimev1alpha1.ProviderCredentials{
										Source: runtimev1alpha1.CredentialsSourceSecret,
									},
								},
							},
						}
						return nil
					}),
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			},
			args: args{
				mg: &v1alpha1.RDSInstance{
					Spec: v1alpha1.RDSInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ProviderConfigReference: &runtimev1alpha1.Reference{},
						},
					},
				},
			},
			want: errors.New(errNoConnectionSecret),
		},
		"GetConnectionSecretError": {
			reason: "Errors getting a secret should be returned",
			fields: fields{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
						switch t := obj.(type) {
						case *corev1.Secret:
							return errBoom
						case *aliv1alpha1.ProviderConfig:
							*t = aliv1alpha1.ProviderConfig{
								Spec: aliv1alpha1.ProviderConfigSpec{
									ProviderConfigSpec: runtimev1alpha1.ProviderConfigSpec{
										Credentials: runtimev1alpha1.ProviderCredentials{
											Source: runtimev1alpha1.CredentialsSourceSecret,
											SecretRef: &runtimev1alpha1.SecretKeySelector{
												SecretReference: runtimev1alpha1.SecretReference{
													Name: "coolsecret",
												},
											},
										},
									},
								},
							}
						}
						return nil
					}),
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			},
			args: args{
				mg: &v1alpha1.RDSInstance{
					Spec: v1alpha1.RDSInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ProviderConfigReference: &runtimev1alpha1.Reference{},
						},
					},
				},
			},
			want: errors.Wrap(errBoom, errGetConnectionSecret),
		},
		"NewRDSClientError": {
			reason: "Errors getting a secret should be returned",
			fields: fields{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
						if t, ok := obj.(*aliv1alpha1.ProviderConfig); ok {
							*t = aliv1alpha1.ProviderConfig{
								Spec: aliv1alpha1.ProviderConfigSpec{
									ProviderConfigSpec: runtimev1alpha1.ProviderConfigSpec{
										Credentials: runtimev1alpha1.ProviderCredentials{
											Source: runtimev1alpha1.CredentialsSourceSecret,
											SecretRef: &runtimev1alpha1.SecretKeySelector{
												SecretReference: runtimev1alpha1.SecretReference{
													Name: "coolsecret",
												},
											},
										},
									},
								},
							}
						}
						return nil
					}),
				},
				usage: resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
				newRDSClient: func(ctx context.Context, accessKeyID, accessKeySecret, region string) (rds.Client, error) {
					return nil, errBoom
				},
			},
			args: args{
				mg: &v1alpha1.RDSInstance{
					Spec: v1alpha1.RDSInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ProviderConfigReference: &runtimev1alpha1.Reference{},
						},
					},
				},
			},
			want: errors.Wrap(errBoom, errCreateRDSClient),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &connector{client: tc.fields.client, usage: tc.fields.usage, newRDSClient: tc.fields.newRDSClient}
			_, err := c.Connect(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nc.Connect(...) -want error, +got error:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestExternalClientObserve(t *testing.T) {
	e := &external{client: &fakeRDSClient{}}
	obj := &v1alpha1.RDSInstance{
		Spec: v1alpha1.RDSInstanceSpec{
			ForProvider: v1alpha1.RDSInstanceParameters{
				MasterUsername: testName,
			},
		},
		Status: v1alpha1.RDSInstanceStatus{
			AtProvider: v1alpha1.RDSInstanceObservation{
				DBInstanceID: testName,
			},
		},
	}
	ob, err := e.Observe(context.Background(), obj)
	if err != nil {
		t.Fatal(err)
	}
	if obj.Status.AtProvider.DBInstanceStatus != v1alpha1.RDSInstanceStateRunning {
		t.Errorf("DBInstanceStatus (%v) should be %v", obj.Status.AtProvider.DBInstanceStatus, v1alpha1.RDSInstanceStateRunning)
	}
	if obj.Status.AtProvider.AccountReady != true {
		t.Error("AccountReady should be true")
	}
	if string(ob.ConnectionDetails[runtimev1alpha1.ResourceCredentialsSecretUserKey]) != testName {
		t.Error("ConnectionDetails should include username=test")
	}
}

func TestExternalClientCreate(t *testing.T) {
	e := &external{client: &fakeRDSClient{}}
	obj := &v1alpha1.RDSInstance{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				crossplanemeta.AnnotationKeyExternalName: testName,
			},
		},
		Spec: v1alpha1.RDSInstanceSpec{
			ForProvider: v1alpha1.RDSInstanceParameters{
				MasterUsername:        testName,
				Engine:                "PostgreSQL",
				EngineVersion:         "10.0",
				SecurityIPList:        "0.0.0.0/0",
				DBInstanceClass:       "rds.pg.s1.small",
				DBInstanceStorageInGB: 20,
			},
		},
	}
	ob, err := e.Create(context.Background(), obj)
	if err != nil {
		t.Fatal(err)
	}
	if obj.Status.AtProvider.DBInstanceID != testName {
		t.Error("DBInstanceID should be set to 'test'")
	}
	if string(ob.ConnectionDetails[runtimev1alpha1.ResourceCredentialsSecretEndpointKey]) != "172.0.0.1" ||
		string(ob.ConnectionDetails[runtimev1alpha1.ResourceCredentialsSecretPortKey]) != "8888" {
		t.Error("ConnectionDetails should include endpoint=172.0.0.1 and port=8888")
	}
}

func TestExternalClientDelete(t *testing.T) {
	e := &external{client: &fakeRDSClient{}}
	obj := &v1alpha1.RDSInstance{
		Status: v1alpha1.RDSInstanceStatus{
			AtProvider: v1alpha1.RDSInstanceObservation{
				DBInstanceID: testName,
			},
		},
	}
	err := e.Delete(context.Background(), obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetConnectionDetails(t *testing.T) {
	address := "0.0.0.0"
	port := "3346"
	password := "super-secret"

	type args struct {
		pw string
		cr *v1alpha1.RDSInstance
		i  *rds.DBInstance
	}
	type want struct {
		conn managed.ConnectionDetails
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessfulNoPassword": {
			args: args{
				pw: "",
				cr: &v1alpha1.RDSInstance{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							crossplanemeta.AnnotationKeyExternalName: testName,
						},
					},
					Spec: v1alpha1.RDSInstanceSpec{
						ForProvider: v1alpha1.RDSInstanceParameters{
							MasterUsername: testName,
						},
					},
				},
				i: &rds.DBInstance{
					Endpoint: &v1alpha1.Endpoint{
						Address: address,
						Port:    port,
					},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					runtimev1alpha1.ResourceCredentialsSecretUserKey:     []byte(testName),
					runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(address),
					runtimev1alpha1.ResourceCredentialsSecretPortKey:     []byte(port),
				},
			},
		},
		"SuccessfulNoEndpoint": {
			args: args{
				pw: password,
				cr: &v1alpha1.RDSInstance{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							crossplanemeta.AnnotationKeyExternalName: testName,
						},
					},
					Spec: v1alpha1.RDSInstanceSpec{
						ForProvider: v1alpha1.RDSInstanceParameters{
							MasterUsername: testName,
						},
					},
				},
				i: &rds.DBInstance{},
			},
			want: want{
				conn: managed.ConnectionDetails{
					runtimev1alpha1.ResourceCredentialsSecretUserKey:     []byte(testName),
					runtimev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password),
				},
			},
		},
		"Successful": {
			args: args{
				pw: password,
				cr: &v1alpha1.RDSInstance{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							crossplanemeta.AnnotationKeyExternalName: testName,
						},
					},
					Spec: v1alpha1.RDSInstanceSpec{
						ForProvider: v1alpha1.RDSInstanceParameters{
							MasterUsername: testName,
						},
					},
				},
				i: &rds.DBInstance{
					Endpoint: &v1alpha1.Endpoint{
						Address: address,
						Port:    port,
					},
				},
			},
			want: want{
				conn: managed.ConnectionDetails{
					runtimev1alpha1.ResourceCredentialsSecretUserKey:     []byte(testName),
					runtimev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password),
					runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(address),
					runtimev1alpha1.ResourceCredentialsSecretPortKey:     []byte(port),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			conn := getConnectionDetails(tc.args.pw, tc.args.cr, tc.args.i)
			if diff := cmp.Diff(tc.want.conn, conn); diff != "" {
				t.Errorf("getConnectionDetails(...): -want, +got:\n%s", diff)
			}
		})
	}
}

type fakeRDSClient struct {
}

func (c *fakeRDSClient) DescribeDBInstance(id string) (*rds.DBInstance, error) {
	if id != testName {
		return nil, errors.New("DescribeDBInstance: client doesn't work")
	}
	return &rds.DBInstance{
		ID:     id,
		Status: v1alpha1.RDSInstanceStateRunning,
	}, nil
}

func (c *fakeRDSClient) CreateDBInstance(req *rds.CreateDBInstanceRequest) (*rds.DBInstance, error) {
	if req.Name != testName || req.Engine != "PostgreSQL" {
		return nil, errors.New("CreateDBInstance: client doesn't work")
	}
	return &rds.DBInstance{
		ID: testName,
		Endpoint: &v1alpha1.Endpoint{
			Address: "172.0.0.1",
			Port:    "8888",
		},
	}, nil
}

func (c *fakeRDSClient) CreateAccount(id, user, pw string) error {
	if id != testName {
		return errors.New("CreateAccount: client doesn't work")
	}
	return nil
}

func (c *fakeRDSClient) DeleteDBInstance(id string) error {
	if id != testName {
		return errors.New("DeleteDBInstance: client doesn't work")
	}
	return nil
}
