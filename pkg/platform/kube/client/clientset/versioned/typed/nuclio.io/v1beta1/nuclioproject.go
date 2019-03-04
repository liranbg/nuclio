/*
Copyright The Kubernetes Authors.

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

package v1beta1

import (
	"time"

	v1beta1 "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	scheme "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// NuclioProjectsGetter has a method to return a NuclioProjectInterface.
// A group's client should implement this interface.
type NuclioProjectsGetter interface {
	NuclioProjects(namespace string) NuclioProjectInterface
}

// NuclioProjectInterface has methods to work with NuclioProject resources.
type NuclioProjectInterface interface {
	Create(*v1beta1.NuclioProject) (*v1beta1.NuclioProject, error)
	Update(*v1beta1.NuclioProject) (*v1beta1.NuclioProject, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.NuclioProject, error)
	List(opts v1.ListOptions) (*v1beta1.NuclioProjectList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.NuclioProject, err error)
	NuclioProjectExpansion
}

// nuclioProjects implements NuclioProjectInterface
type nuclioProjects struct {
	client rest.Interface
	ns     string
}

// newNuclioProjects returns a NuclioProjects
func newNuclioProjects(c *NuclioV1beta1Client, namespace string) *nuclioProjects {
	return &nuclioProjects{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the nuclioProject, and returns the corresponding nuclioProject object, and an error if there is any.
func (c *nuclioProjects) Get(name string, options v1.GetOptions) (result *v1beta1.NuclioProject, err error) {
	result = &v1beta1.NuclioProject{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("nuclioprojects").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of NuclioProjects that match those selectors.
func (c *nuclioProjects) List(opts v1.ListOptions) (result *v1beta1.NuclioProjectList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1beta1.NuclioProjectList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("nuclioprojects").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested nuclioProjects.
func (c *nuclioProjects) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("nuclioprojects").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a nuclioProject and creates it.  Returns the server's representation of the nuclioProject, and an error, if there is any.
func (c *nuclioProjects) Create(nuclioProject *v1beta1.NuclioProject) (result *v1beta1.NuclioProject, err error) {
	result = &v1beta1.NuclioProject{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("nuclioprojects").
		Body(nuclioProject).
		Do().
		Into(result)
	return
}

// Update takes the representation of a nuclioProject and updates it. Returns the server's representation of the nuclioProject, and an error, if there is any.
func (c *nuclioProjects) Update(nuclioProject *v1beta1.NuclioProject) (result *v1beta1.NuclioProject, err error) {
	result = &v1beta1.NuclioProject{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("nuclioprojects").
		Name(nuclioProject.Name).
		Body(nuclioProject).
		Do().
		Into(result)
	return
}

// Delete takes name of the nuclioProject and deletes it. Returns an error if one occurs.
func (c *nuclioProjects) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("nuclioprojects").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *nuclioProjects) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("nuclioprojects").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched nuclioProject.
func (c *nuclioProjects) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.NuclioProject, err error) {
	result = &v1beta1.NuclioProject{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("nuclioprojects").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
