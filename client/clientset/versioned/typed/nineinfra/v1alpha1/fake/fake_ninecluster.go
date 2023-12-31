/*
Copyright 2023 nineinfra.

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
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNineClusters implements NineClusterInterface
type FakeNineClusters struct {
	Fake *FakeNineinfraV1alpha1
	ns   string
}

var nineclustersResource = v1alpha1.GroupVersion.WithResource("nineclusters")

var nineclustersKind = v1alpha1.GroupVersion.WithKind("NineCluster")

// Get takes name of the nineCluster, and returns the corresponding nineCluster object, and an error if there is any.
func (c *FakeNineClusters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.NineCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(nineclustersResource, c.ns, name), &v1alpha1.NineCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NineCluster), err
}

// List takes label and field selectors, and returns the list of NineClusters that match those selectors.
func (c *FakeNineClusters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.NineClusterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(nineclustersResource, nineclustersKind, c.ns, opts), &v1alpha1.NineClusterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.NineClusterList{ListMeta: obj.(*v1alpha1.NineClusterList).ListMeta}
	for _, item := range obj.(*v1alpha1.NineClusterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested nineClusters.
func (c *FakeNineClusters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(nineclustersResource, c.ns, opts))

}

// Create takes the representation of a nineCluster and creates it.  Returns the server's representation of the nineCluster, and an error, if there is any.
func (c *FakeNineClusters) Create(ctx context.Context, nineCluster *v1alpha1.NineCluster, opts v1.CreateOptions) (result *v1alpha1.NineCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(nineclustersResource, c.ns, nineCluster), &v1alpha1.NineCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NineCluster), err
}

// Update takes the representation of a nineCluster and updates it. Returns the server's representation of the nineCluster, and an error, if there is any.
func (c *FakeNineClusters) Update(ctx context.Context, nineCluster *v1alpha1.NineCluster, opts v1.UpdateOptions) (result *v1alpha1.NineCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(nineclustersResource, c.ns, nineCluster), &v1alpha1.NineCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NineCluster), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeNineClusters) UpdateStatus(ctx context.Context, nineCluster *v1alpha1.NineCluster, opts v1.UpdateOptions) (*v1alpha1.NineCluster, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(nineclustersResource, "status", c.ns, nineCluster), &v1alpha1.NineCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NineCluster), err
}

// Delete takes name of the nineCluster and deletes it. Returns an error if one occurs.
func (c *FakeNineClusters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(nineclustersResource, c.ns, name, opts), &v1alpha1.NineCluster{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNineClusters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(nineclustersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.NineClusterList{})
	return err
}

// Patch applies the patch and returns the patched nineCluster.
func (c *FakeNineClusters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.NineCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(nineclustersResource, c.ns, name, pt, data, subresources...), &v1alpha1.NineCluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NineCluster), err
}
