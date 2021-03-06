// Copyright 2020 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
)

type CatalogCreator interface {
	CreateCatalog(ctx context.Context, name string) (*v1alpha1.CatalogSource, error)
}

type CatalogUpdater interface {
	UpdateCatalog(ctx context.Context, cs *v1alpha1.CatalogSource, subscription *v1alpha1.Subscription) error
}
