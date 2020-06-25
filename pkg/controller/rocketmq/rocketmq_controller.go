package rocketmq

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	middlewarev1alpha1 "github.com/riete/rocketmq-operator/pkg/apis/middleware/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_rocketmq")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new RocketMQ Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRocketMQ{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("rocketmq-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource RocketMQ
	err = c.Watch(&source.Kind{Type: &middlewarev1alpha1.RocketMQ{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner RocketMQ
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &middlewarev1alpha1.RocketMQ{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileRocketMQ implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileRocketMQ{}

// ReconcileRocketMQ reconciles a RocketMQ object
type ReconcileRocketMQ struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a RocketMQ object and makes changes based on the state read
// and what is in the RocketMQ.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileRocketMQ) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RocketMQ")

	// Fetch the RocketMQ instance
	instance := &middlewarev1alpha1.RocketMQ{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Create Services
	srvService := newService(instance, middlewarev1alpha1.NamesrvRole)
	masterService := newService(instance, middlewarev1alpha1.MasterRole)
	slaveService := newService(instance, middlewarev1alpha1.SlaveRole)
	services := []*corev1.Service{srvService, masterService, slaveService}
	if instance.Spec.Console {
		services = append(services, newService(instance, middlewarev1alpha1.ConsoleRole))
	}

	for _, service := range services {
		// Set RocketMQ instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, service, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Check if this service already exists
		found := &corev1.Service{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
			err = r.client.Create(context.TODO(), service)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	srvStatefulSet := newNameSrvStatefulSet(instance)
	masterStatefulSet := newBrokerStatefulSet(instance, middlewarev1alpha1.MasterRole)
	slaveStatefulSet := newBrokerStatefulSet(instance, middlewarev1alpha1.SlaveRole)
	statefulSets := []*appsv1.StatefulSet{srvStatefulSet, masterStatefulSet, slaveStatefulSet}

	requeue := false
	for _, statefulSet := range statefulSets {
		// Set RocketMQ instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, statefulSet, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Check if this service already exists
		found := &appsv1.StatefulSet{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: statefulSet.Name, Namespace: statefulSet.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new StatefulSet", "StatefulSet.Namespace", statefulSet.Namespace, "StatefulSet.Name", statefulSet.Name)
			err = r.client.Create(context.TODO(), statefulSet)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// ensure broker replicas size
		if found.Spec.Replicas != nil && found.Name != fmt.Sprintf("%s-%s", instance.Name, middlewarev1alpha1.NamesrvRole) {
			if *found.Spec.Replicas != instance.Spec.MasterSlave.Num {
				found.Spec.Replicas = &instance.Spec.MasterSlave.Num
				err = r.client.Update(context.TODO(), found)
				requeue = true
				if err != nil {
					reqLogger.Error(err, "Failed to update StatefulSet", "StatefulSet.Namespace", found.Namespace, "StatefulSet.Name", found.Name)
					return reconcile.Result{}, err
				}
				reqLogger.Info("Update StatefulSet", "StatefulSet.Namespace", found.Namespace, "StatefulSet.Name", found.Name)
			}
		}
	}

	if instance.Spec.Console {
		console := newConsole(instance)
		if err := controllerutil.SetControllerReference(instance, console, r.scheme); err != nil {
			return reconcile.Result{}, err
		}
		consoleFound := &appsv1.Deployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: console.Name, Namespace: console.Namespace}, consoleFound)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new deployment", "Deployment.Namespace", console.Namespace, "Deployment.Name", console.Name)
			err = r.client.Create(context.TODO(), console)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
	}
	// MQ already exists - don't requeue
	reqLogger.Info("Skip reconcile: RocketMQ already exists", "RocketMQ.Namespace", instance.Namespace, "RocketMQ.Name", instance.Name)
	return reconcile.Result{Requeue: requeue}, nil
}

func newConsole(mq *middlewarev1alpha1.RocketMQ) *appsv1.Deployment {
	var nameSrvAddr []string
	nameSrvServiceName := fmt.Sprintf("%s-%s", mq.Name, middlewarev1alpha1.NamesrvRole)
	nameSrvStatefulSetName := fmt.Sprintf("%s-%s", mq.Name, middlewarev1alpha1.NamesrvRole)
	for i := 0; i < int(mq.Spec.NameSrv.Num); i++ {
		nameSrvAddr = append(nameSrvAddr, fmt.Sprintf(
			"%s-%d.%s:%d",
			nameSrvStatefulSetName,
			i,
			nameSrvServiceName,
			middlewarev1alpha1.NamesrvPort),
		)
	}
	var replicas int32 = 1

	cpuRequest, _ := resource.ParseQuantity("50m")
	cpuLimit, _ := resource.ParseQuantity("200m")
	memoryRequest, _ := resource.ParseQuantity("512Mi")
	memoryLimit, _ := resource.ParseQuantity("1024Mi")
	name := fmt.Sprintf("%s-%s", mq.Name, middlewarev1alpha1.ConsoleRole)
	labels := map[string]string{"app": name}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mq.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: middlewarev1alpha1.ConsoleImage,
						Name:  name,
						Env: []corev1.EnvVar{{
							Name: "JAVA_OPTS",
							Value: fmt.Sprintf(
								"-Xmx512m -Xms256m -Drocketmq.namesrv.addr=%s -Dcom.rocketmq.sendMessageWithVIPChannel=false",
								strings.Join(nameSrvAddr, ";"),
							),
						}},
						Ports: []corev1.ContainerPort{{
							ContainerPort: middlewarev1alpha1.ConsolePort,
							Name:          "console",
							Protocol:      corev1.ProtocolTCP,
						}},
						LivenessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 120,
							PeriodSeconds:       30,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(middlewarev1alpha1.ConsolePort)),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 120,
							PeriodSeconds:       30,
							SuccessThreshold:    1,
							TimeoutSeconds:      3,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(middlewarev1alpha1.ConsolePort)),
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    cpuRequest,
								corev1.ResourceMemory: memoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    cpuLimit,
								corev1.ResourceMemory: memoryLimit,
							},
						},
					}},
				},
			},
		},
	}
	return deployment
}

func newService(mq *middlewarev1alpha1.RocketMQ, role middlewarev1alpha1.RoleName) *corev1.Service {
	name := fmt.Sprintf("%s-%s", mq.Name, role)
	labels := map[string]string{"app": name}
	clusterIP := "None"

	var port int32
	if role == middlewarev1alpha1.NamesrvRole {
		port = middlewarev1alpha1.NamesrvPort
	} else if role == middlewarev1alpha1.MasterRole || role == middlewarev1alpha1.SlaveRole {
		port = middlewarev1alpha1.BrokerPort
	} else if role == middlewarev1alpha1.ConsoleRole {
		port = middlewarev1alpha1.ConsolePort
		clusterIP = ""
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mq.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: clusterIP,
			Selector:  labels,
			Type:      corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       port,
				TargetPort: intstr.FromInt(int(port)),
			}},
		},
	}
}

func newBrokerStatefulSet(mq *middlewarev1alpha1.RocketMQ, role middlewarev1alpha1.RoleName) *appsv1.StatefulSet {
	var nameSrvAddr []string
	nameSrvServiceName := fmt.Sprintf("%s-%s", mq.Name, middlewarev1alpha1.NamesrvRole)
	nameSrvStatefulSetName := fmt.Sprintf("%s-%s", mq.Name, middlewarev1alpha1.NamesrvRole)
	for i := 0; i < int(mq.Spec.NameSrv.Num); i++ {
		nameSrvAddr = append(nameSrvAddr, fmt.Sprintf(
			"%s-%d.%s:%d",
			nameSrvStatefulSetName,
			i,
			nameSrvServiceName,
			middlewarev1alpha1.NamesrvPort),
		)
	}

	image := fmt.Sprintf("%s-%s-%s", middlewarev1alpha1.BrokerBaseImage, role, mq.Spec.MasterSlave.Mode)
	name := fmt.Sprintf("%s-%s", mq.Name, role)
	labels := map[string]string{"app": name}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mq.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &mq.Spec.MasterSlave.Num,
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: image,
						Name:  name,
						Ports: []corev1.ContainerPort{{
							ContainerPort: middlewarev1alpha1.BrokerPort,
							Name:          "broker",
							Protocol:      corev1.ProtocolTCP,
						}},
						Env: []corev1.EnvVar{
							{Name: "JVM_XMX", Value: mq.Spec.MasterSlave.Xmx},
							{Name: "JVM_XMN", Value: mq.Spec.MasterSlave.Xmn},
							{Name: "JVM_MAX_DIRECT_MEMORY_SIZE", Value: mq.Spec.MasterSlave.MaxDirectMemorySize},
							{Name: "NAMESRV", Value: strings.Join(nameSrvAddr, ";")},
						},
						LivenessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 60,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      10,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(middlewarev1alpha1.BrokerPort)),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 60,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      10,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(middlewarev1alpha1.BrokerPort)),
								},
							},
						},
						Resources: mq.Spec.MasterSlave.Resources,
					}},
				},
			},
		},
	}
	if mq.Spec.MasterSlave.StorageClass != "" {
		storageSize, _ := resource.ParseQuantity(mq.Spec.MasterSlave.StorageSize)
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{Name: "data", MountPath: "/app/rocketmq-data"},
		}
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "data",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageSize,
					},
				},
				StorageClassName: &mq.Spec.MasterSlave.StorageClass,
			},
		}}
	}
	return sts
}

func newNameSrvStatefulSet(mq *middlewarev1alpha1.RocketMQ) *appsv1.StatefulSet {
	name := fmt.Sprintf("%s-%s", mq.Name, middlewarev1alpha1.NamesrvRole)
	image := middlewarev1alpha1.NameSrvImage
	labels := map[string]string{"app": name}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mq.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &mq.Spec.NameSrv.Num,
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: image,
						Name:  name,
						Ports: []corev1.ContainerPort{{
							ContainerPort: middlewarev1alpha1.NamesrvPort,
							Name:          "namesrv",
							Protocol:      corev1.ProtocolTCP,
						}},
						Env: []corev1.EnvVar{
							{Name: "JVM_XMX", Value: mq.Spec.NameSrv.Xmx},
							{Name: "JVM_XMN", Value: mq.Spec.NameSrv.Xmn},
						},
						LivenessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 30,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      10,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(middlewarev1alpha1.NamesrvPort)),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							FailureThreshold:    3,
							InitialDelaySeconds: 30,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							TimeoutSeconds:      10,
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromInt(int(middlewarev1alpha1.NamesrvPort)),
								},
							},
						},
						Resources: mq.Spec.NameSrv.Resources,
					}},
				},
			},
		},
	}
}
