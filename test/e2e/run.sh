#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#

#!/bin/bash

cd "$(dirname "$0")" || exit

KUBECONFIG=${KUBECONFIG:=${HOME}/.kube/config} 

if [ ! -f "$KUBECONFIG" ]; then
    echo "Kubeconfig $KUBECONFIG not found!"
    exit 1
fi

if [ -z "$DISK_OFFERING_ID" ]; then
    echo "Variable DISK_OFFERING_ID not set!"
    exit 1
fi

# Create storage class "cloudstack-csi-driver-e2e"
scName="cloudstack-csi-driver-e2e"
sed "s/<disk-offering-id>/${DISK_OFFERING_ID}/" storageclass.yaml | kubectl apply -f -

# Run in parallel when possible (exclude [Feature:.*], [Disruptive] and [Serial]):
./ginkgo -p -progress -v \
       -focus='External.Storage.*csi-cloudstack' \
       -skip='\[Feature:|\[Disruptive\]|\[Serial\]' \
       e2e.test -- \
       -storage.testdriver=testdriver.yaml \
        --kubeconfig="$KUBECONFIG"

# Delete volume populators CRD created by e2e.test in the previous run
# This prevents a test from failing with CRD already exists
kubectl delete crd volumepopulators.populator.storage.k8s.io

# Then run the remaining tests, sequentially:
./ginkgo -progress -v \
       -focus='External.Storage.*csi-cloudstack.*(\[Feature:|\[Disruptive\]|\[Serial\])' \
       e2e.test -- \
       -storage.testdriver=testdriver.yaml \
       --kubeconfig="$KUBECONFIG"

# Delete storage class
kubectl delete storageclasses.storage.k8s.io "${scName}" || echo "No storage class named ${scName}"