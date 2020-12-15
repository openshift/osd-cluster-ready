module github.com/iamkirkbater/osd-readiness-spike

go 1.15

require (
	github.com/openshift/api v0.0.0-20200521101457-60c476765272
	github.com/openshift/client-go v0.0.0-00010101000000-000000000000
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.18.3
)

replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20200526144822-34f54f12813a
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200521150516-05eb9880269c
)
