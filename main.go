package main

import (
	"flag"
	"fmt"
	gamekruiseiov1alpha1 "github.com/openkruise/kruise-game/apis/v1alpha1"
	kruisegameclientset "github.com/openkruise/kruise-game/pkg/client/clientset/versioned"
	"github.com/openkruise/kruise-game/pkg/util"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"strconv"
	"strings"
	"time"
)

type input struct {
	gssName                 string
	namespace               string
	timeout                 int
	selectIds               string
	selectOpsState          string
	selectNetworkDisabled   string
	selectNotContainerImage string
	expOpsState             string
	expNetworkDisabled      string
}

type selectOption struct {
	gssNames          []string
	namespace         string
	gsNames           []string
	opsState          *gamekruiseiov1alpha1.OpsState
	networkDisabled   *bool
	notContainerImage map[string]string
}

type expectOption struct {
	namespace       string
	opsState        *gamekruiseiov1alpha1.OpsState
	networkDisabled *bool
}

func main() {
	i := input{}
	flag.StringVar(&i.gssName, "gss-name", "", "gssName")
	flag.StringVar(&i.namespace, "namespace", "", "namespace")
	flag.IntVar(&i.timeout, "timeout", 300, "timeout")
	flag.StringVar(&i.selectIds, "select-ids", "", "selectIds")
	flag.StringVar(&i.selectOpsState, "select-opsState", "", "selectOpsState")
	flag.StringVar(&i.selectNetworkDisabled, "select-networkDisabled", "", "selectNetworkDisabled")
	flag.StringVar(&i.selectNotContainerImage, "select-not-container-image", "", "selectNotContainerImage")
	flag.StringVar(&i.expOpsState, "exp-opsState", "", "expOpsState")
	flag.StringVar(&i.expNetworkDisabled, "exp-networkDisabled", "", "expNetworkDisabled")
	flag.Parse()

	ctx := context.Background()
	cfg := config.GetConfigOrDie()
	kruisegameClient := kruisegameclientset.NewForConfigOrDie(cfg)
	kubeClient := clientset.NewForConfigOrDie(cfg)

	so, err := consSelectOption(ctx, kruisegameClient, i)
	if err != nil {
		panic(err)
	}
	fmt.Printf("selectOption: %+v\n", so)

	eo, err := consExpectOption(i)
	if err != nil {
		panic(err)
	}
	fmt.Printf("expectOption: %+v\n", eo)

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, time.Duration(i.timeout)*time.Second, true, func(ctx context.Context) (done bool, err error) {
		selectedGameServers, err := SelectGameServers(ctx, kruisegameClient, kubeClient, so)
		if err != nil {
			return false, err
		}

		if err = UpdateGameServers(ctx, kruisegameClient, selectedGameServers, eo); err != nil {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		panic(fmt.Errorf("update GameServers failed, err:%v\n", err))
	}
	fmt.Printf("update GameServers Done\n")
}

func UpdateGameServers(ctx context.Context, kgClient *kruisegameclientset.Clientset, gsList []gamekruiseiov1alpha1.GameServer, eo *expectOption) error {
	for _, gs := range gsList {
		gsNew := gs.DeepCopy()

		// set networkDisabled
		if eo.networkDisabled != nil {
			gsNew.Spec.NetworkDisabled = *eo.networkDisabled
		}

		// set opsState
		if eo.opsState != nil {
			gsNew.Spec.OpsState = *eo.opsState
		}

		_, err := kgClient.GameV1alpha1().GameServers(gs.Namespace).Update(ctx, gsNew, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func SelectGameServers(ctx context.Context, kgClient kruisegameclientset.Interface, kubeClient clientset.Interface, so *selectOption) ([]gamekruiseiov1alpha1.GameServer, error) {
	var selectedGameServers []gamekruiseiov1alpha1.GameServer
	var selectNames []string

	for _, gssName := range so.gssNames {
		labelSelector := labels.SelectorFromSet(map[string]string{
			gamekruiseiov1alpha1.GameServerOwnerGssKey: gssName,
		}).String()
		gsList, err := kgClient.GameV1alpha1().GameServers(so.namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return nil, err
		}

		for _, gs := range gsList.Items {
			// filter by idList
			if len(so.gsNames) > 0 && !util.IsStringInList(gs.Name, so.gsNames) {
				continue
			}

			// filter by opsState
			if so.opsState != nil && gs.Spec.OpsState != *so.opsState {
				continue
			}

			// filter by networkDisabled
			if so.networkDisabled != nil && *so.networkDisabled != gs.Spec.NetworkDisabled {
				continue
			}

			// filter by containerImage
			if so.notContainerImage != nil {
				pod, err := kubeClient.CoreV1().Pods(so.namespace).Get(ctx, gs.Name, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
				actual := getContainerImage(pod.DeepCopy())

				hit := false
				for container, image := range so.notContainerImage {
					if actual[container] == image {
						hit = true
						break
					}
				}
				if hit {
					continue
				}
			}

			selectedGameServers = append(selectedGameServers, gs)
			selectNames = append(selectNames, gs.Name)
		}
	}

	fmt.Printf("select GameServers Names: %v\n", selectNames)
	return selectedGameServers, nil
}

func consExpectOption(i input) (*expectOption, error) {
	// parse expOpsState
	var opsState *gamekruiseiov1alpha1.OpsState
	if i.expOpsState != "" {
		opsStateTemp := (gamekruiseiov1alpha1.OpsState)(i.expOpsState)
		opsState = &opsStateTemp
	}

	// parse expNetworkDisabled
	var networkDisabled *bool
	if i.expNetworkDisabled != "" {
		networkDisabledTemp := strings.ToLower(i.expNetworkDisabled) == "true"
		networkDisabled = &networkDisabledTemp
	}

	return &expectOption{
		namespace:       i.namespace,
		opsState:        opsState,
		networkDisabled: networkDisabled,
	}, nil
}

func consSelectOption(ctx context.Context, kgClient kruisegameclientset.Interface, i input) (*selectOption, error) {
	// parse gssNames
	var gssNames []string
	if i.gssName == "" {
		gssList, err := kgClient.GameV1alpha1().GameServerSets(i.namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, gss := range gssList.Items {
			gssNames = append(gssNames, gss.Name)
		}
	} else {
		gssNames = strings.Split(i.gssName, ",")
	}

	// parse selectIds
	var gsNames []string
	if i.selectIds != "" {
		for _, gssName := range gssNames {
			for _, idStr := range strings.Split(i.selectIds, ",") {
				idInt, err := strconv.Atoi(idStr)
				if err != nil {
					return nil, err
				}
				if idInt < 0 {
					return nil, fmt.Errorf("invalid id %s", idStr)
				}
				gsName := gssName + "-" + idStr
				gsNames = append(gsNames, gsName)
			}
		}
	}

	// parse selectOpsState
	var opsState *gamekruiseiov1alpha1.OpsState
	if i.selectOpsState != "" {
		opsStateTemp := (gamekruiseiov1alpha1.OpsState)(i.selectOpsState)
		opsState = &opsStateTemp
	}

	// parse selectNetworkDisabled
	var networkDisabled *bool
	if i.selectNetworkDisabled != "" {
		networkDisabledTemp := strings.ToLower(i.selectNetworkDisabled) == "true"
		networkDisabled = &networkDisabledTemp
	}

	// parse selectNotContainerImage
	var notContainerImage map[string]string
	if i.selectNotContainerImage != "" {
		for _, snci := range strings.Split(i.selectNotContainerImage, ",") {
			container := strings.Split(snci, "/")[0]
			image, _ := strings.CutPrefix(snci, container+"/")
			notContainerImage = make(map[string]string)
			notContainerImage[container] = image
		}
	}

	return &selectOption{
		gssNames:          gssNames,
		namespace:         i.namespace,
		gsNames:           gsNames,
		opsState:          opsState,
		networkDisabled:   networkDisabled,
		notContainerImage: notContainerImage,
	}, nil
}

func getContainerImage(pod *corev1.Pod) map[string]string {
	containerImage := make(map[string]string)
	for _, container := range pod.Spec.Containers {
		containerImage[container.Name] = container.Image
	}
	return containerImage
}
