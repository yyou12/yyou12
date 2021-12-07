FROM registry.ci.openshift.org/ocp/builder:rhel-8-golang-1.16-openshift-4.10 AS builder
RUN mkdir -p /go/src/github.com/openshift/openshift-tests-private
WORKDIR /go/src/github.com/openshift/openshift-tests-private
COPY . .
RUN make build; \
    mkdir -p /tmp/build; \
    cp /go/src/github.com/openshift/openshift-tests-private/bin/extended-platform-tests /tmp/build/extended-platform-tests


FROM registry.ci.openshift.org/ocp/4.10:tools
COPY --from=builder /tmp/build/extended-platform-tests /usr/bin/
RUN PACKAGES="git gzip zip util-linux openssh-clients httpd-tools nodejs" && \
    yum install --setopt=tsflags=nodocs -y $PACKAGES && yum clean all && rm -rf /var/cache/yum/* && \
    git config --system user.name test-private && \
    git config --system user.email test-private@test.com && \
    chmod g+w /etc/passwd
RUN pip3 install dotmap minio pyyaml requests
RUN curl -o- -L https://yarnpkg.com/install.sh | bash && \
    curl -s -k https://mirror2.openshift.com/pub/openshift-v4/clients/ocp-dev-preview/latest-4.10/opm-linux.tar.gz -o opm-linux.tar.gz && \
    tar -C /usr/bin -xzvf opm-linux.tar.gz && \
    rm -fr opm-linux.tar.gz && \
    curl -s -k https://mirror2.openshift.com/pub/openshift-v4/x86_64/clients/operator-sdk/ -o sdk.html && \
    optsdkver=$(grep -E "<a href=\"4\.9" sdk.html |cut -d"\"" -f2|cut -d"/" -f1|sort -V|tail -1) && \
    echo ${optsdkver} && \
    curl -s -k https://mirror2.openshift.com/pub/openshift-v4/x86_64/clients/operator-sdk/${optsdkver}/operator-sdk-linux-x86_64.tar.gz -o opt-sdk.tar.gz && \
    tar -C /usr/bin -xzvf opt-sdk.tar.gz && \
    rm -fr sdk.html opt-sdk.tar.gz /usr/bin/oc install.sh

