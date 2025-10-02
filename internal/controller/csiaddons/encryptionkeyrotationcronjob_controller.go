/*
Copyright 2024 The Kubernetes-CSI-Addons Authors.

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

package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ref "k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// EncryptionKeyRotationCronJobReconciler reconciles a EncryptionKeyRotationCronJob object
type EncryptionKeyRotationCronJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationcronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationcronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationcronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationjobs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *EncryptionKeyRotationCronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the resource
	krcJob := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}
	err := r.Get(ctx, req.NamespacedName, krcJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("encryptionkeyrotationcronjob resource not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if resource is being deleted
	if !krcJob.DeletionTimestamp.IsZero() {
		logger.Info("EncryptionKeyRotationCronJob is being deleted, exiting reconcile", "namespace", krcJob.Namespace, "name", krcJob.Name)
		return ctrl.Result{}, nil
	}

	// Set default values for the optionals
	if krcJob.Spec.FailedJobsHistoryLimit == nil {
		*krcJob.Spec.FailedJobsHistoryLimit = defaultFailedJobsHistoryLimit
	}
	if krcJob.Spec.SuccessfulJobsHistoryLimit == nil {
		*krcJob.Spec.SuccessfulJobsHistoryLimit = defaultSuccessfulJobsHistoryLimit
	}

	var childJobs csiaddonsv1alpha1.EncryptionKeyRotationJobList
	err = r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name})
	if err != nil {
		logger.Error(err, "failed to fetch list of encryptionkeyrotationjob")
		return ctrl.Result{}, err
	}

	childJobsInfo := parseEncryptionKeyRotationJobList(&logger, &childJobs)

	// construct statuses
	if childJobsInfo.mostRecentTime != nil {
		krcJob.Status.LastScheduleTime = &metav1.Time{Time: *childJobsInfo.mostRecentTime}
	} else {
		krcJob.Status.LastScheduleTime = nil
	}
	if childJobsInfo.lastSuccessfulTime != nil {
		krcJob.Status.LastSuccessfulTime = &metav1.Time{Time: *childJobsInfo.lastSuccessfulTime}
	}
	krcJob.Status.Active = nil
	if childJobsInfo.activeJob != nil {
		jobRef, err := ref.GetReference(r.Scheme, childJobsInfo.activeJob)
		if err != nil {
			logger.Error(err, "failed to get reference to the active job", "job", childJobsInfo.activeJob)
		}
		// might be nil, intentional
		krcJob.Status.Active = jobRef
	}

	// Update status
	if err = r.Status().Update(ctx, krcJob); err != nil {
		logger.Error(err, "failed to update status for encryptionkeyrotationcronjob")
		return ctrl.Result{}, err
	}

	// delete failed/successful jobs older than the specified history limit
	r.deleteOldEncryptionKeyRotationJobs(ctx, &logger, childJobsInfo.failedJobs, *krcJob.Spec.FailedJobsHistoryLimit)
	r.deleteOldEncryptionKeyRotationJobs(ctx, &logger, childJobsInfo.successfulJobs, *krcJob.Spec.SuccessfulJobsHistoryLimit)

	// if suspended, return
	if krcJob.Spec.Suspend != nil && *krcJob.Spec.Suspend {
		logger.Info("encryptionkeyrotationcronjob is suspended, skipping scheduling")
		return ctrl.Result{}, nil
	}

	missedRun, nextRun, err := getNextScheduleForKeyRotation(krcJob, time.Now())
	if err != nil {
		logger.Error(err, "failed to get next schedule for jobs", "schedule", krcJob.Spec.Schedule)

		// Either invalid schedule or too many missed starts
		// Do not requeue
		return ctrl.Result{}, nil
	}

	schedResult := ctrl.Result{RequeueAfter: time.Until(nextRun)}
	logger = logger.WithValues("now", time.Now(), "nextRun", nextRun)

	if missedRun.IsZero() {
		logger.Info("no upcoming schedule, requeue with delay until next run")
		return schedResult, nil
	}

	// If missed start deadline, requeue with delay until next run
	logger = logger.WithValues("currentRun", missedRun)
	tooLate := false
	if krcJob.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*krcJob.Spec.StartingDeadlineSeconds) * time.Second).Before(time.Now())
	}
	if tooLate {
		logger.Info("missed starting deadline for last run, requeue with delay till next run")
		return schedResult, nil
	}

	if krcJob.Spec.ConcurrencyPolicy == csiaddonsv1alpha1.ReplaceConcurrent {
		err = r.Delete(ctx, childJobsInfo.activeJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "failed to delete active encryptionkeyrotationjob", "encryptionkeyrotationjob", childJobsInfo.activeJob.Name)
			return ctrl.Result{}, err
		}
	}

	// return if a job is already active and concurrent jobs are forbidden
	if childJobsInfo.activeJob != nil {
		logger.Info("concurrent runs blocked by policy, skipping", "activeJob", childJobsInfo.activeJob.Name)
		return schedResult, nil
	}

	// now we construct the job
	krJob, err := r.constructEncryptionKeyRotationJob(krcJob, missedRun)
	if err != nil {
		logger.Error(err, "failed to construct encryptionkeyrotationjob resource from cron job")
		return ctrl.Result{}, err
	}
	if err = r.Create(ctx, krJob); err != nil {
		logger.Error(err, "failed to create encryptionkeyrotationjob resource")
		return ctrl.Result{}, err
	}
	logger.Info("successfully created encryptionkeyrotationjob resource", "encryptionkeyrotationjob", krJob.Name)

	return schedResult, nil
}

// encrpytionKeyRotationChildJobInfo holds a representation of parsed
// EncryptionKeyRotationJob(s)
type encrpytionKeyRotationChildJobInfo struct {
	activeJob      *csiaddonsv1alpha1.EncryptionKeyRotationJob
	failedJobs     []*csiaddonsv1alpha1.EncryptionKeyRotationJob
	successfulJobs []*csiaddonsv1alpha1.EncryptionKeyRotationJob

	lastSuccessfulTime *time.Time
	mostRecentTime     *time.Time
}

// parseEncryptionKeyRotationJobList processes EncryptionKeyRotationJobList and
// returns encrpytionKeyRotationChildJobInfo
// MARK:- can be a generic function
func parseEncryptionKeyRotationJobList(
	logger *logr.Logger, jobs *csiaddonsv1alpha1.EncryptionKeyRotationJobList) encrpytionKeyRotationChildJobInfo {
	var activeJob *csiaddonsv1alpha1.EncryptionKeyRotationJob
	var successfulJobs, failedJobs []*csiaddonsv1alpha1.EncryptionKeyRotationJob
	var mostRecentTime, lastSuccessfulTime *time.Time

	for i, job := range jobs.Items {
		switch job.Status.Result {
		case "":
			activeJob = &jobs.Items[i]
		case csiaddonsv1alpha1.OperationResultFailed:
			failedJobs = append(failedJobs, &jobs.Items[i])
		case csiaddonsv1alpha1.OperationResultSucceeded:
			successfulJobs = append(successfulJobs, &jobs.Items[i])
			completionTime := jobs.Items[i].Status.CompletionTime
			if lastSuccessfulTime == nil {
				lastSuccessfulTime = &completionTime.Time
			} else if completionTime != nil && lastSuccessfulTime.Before(completionTime.Time) {
				lastSuccessfulTime = &completionTime.Time
			}
		}

		schedTime, err := getScheduledTimeForEKRJob(&job)
		if err != nil {
			logger.Error(err, "Failed to parse schedule time for child encryptionkeyrotationjob", "encryptionkeyrotationjob", job.Name)
			continue
		}
		if schedTime != nil {
			if mostRecentTime == nil {
				mostRecentTime = schedTime
			} else if mostRecentTime.Before(*schedTime) {
				mostRecentTime = schedTime
			}
		}
	}

	return encrpytionKeyRotationChildJobInfo{
		activeJob:          activeJob,
		failedJobs:         failedJobs,
		successfulJobs:     successfulJobs,
		lastSuccessfulTime: lastSuccessfulTime,
		mostRecentTime:     mostRecentTime,
	}
}

// getScheduledTimeForEKRJob extracts the scheduled time from EncryptionKeyRotationJob's annotation
// MARK:- can be a generic function
func getScheduledTimeForEKRJob(krJob *csiaddonsv1alpha1.EncryptionKeyRotationJob) (*time.Time, error) {
	timeRaw := krJob.Annotations[scheduledTimeAnnotation]
	if len(timeRaw) == 0 {
		return nil, nil
	}

	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
	if err != nil {
		return nil, err
	}
	return &timeParsed, nil
}

// deleteOldEncryptionKeyRotationJobs deletes jobs older than the `limit` after
// sorting them on their start times
func (r *EncryptionKeyRotationCronJobReconciler) deleteOldEncryptionKeyRotationJobs(
	ctx context.Context,
	logger *logr.Logger,
	jobs []*csiaddonsv1alpha1.EncryptionKeyRotationJob,
	limit int32) {

	// sort on start time, chronologically
	// jobs with start time as nil come last
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Status.StartTime == nil {
			return jobs[j].Status.StartTime != nil
		}
		return jobs[i].Status.StartTime.Before(jobs[j].Status.StartTime)
	})

	for i, job := range jobs {
		if int32(i) >= int32(len(jobs))-limit {
			break
		}
		err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to delete old job",
				"encryptionKeyRotationJobName", job.Name,
				"state", job.Status.Result)
		} else {
			logger.Info("Successfully deleted old job",
				"encryptionKeyRotationJobName", job.Name,
				"state", job.Status.Result)
		}
	}
}

// getNextScheduleForKeyRotation returns last missed and next time after
// parsing the schedule.
// An error is returned if start is missed more than 100 times
func getNextScheduleForKeyRotation(
	krcJob *csiaddonsv1alpha1.EncryptionKeyRotationCronJob,
	now time.Time) (time.Time, time.Time, error) {
	sched, err := cron.ParseStandard(krcJob.Spec.Schedule)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("unparsable schedule %q: %v", krcJob.Spec.Schedule, err)
	}

	var earliestTime time.Time
	if krcJob.Status.LastScheduleTime != nil {
		earliestTime = krcJob.Status.LastScheduleTime.Time
	} else {
		earliestTime = krcJob.CreationTimestamp.Time
	}
	if krcJob.Spec.StartingDeadlineSeconds != nil {
		// controller is not going to schedule anything below this point
		schedulingDeadline := now.Add(-time.Second * time.Duration(*krcJob.Spec.StartingDeadlineSeconds))

		if schedulingDeadline.After(earliestTime) {
			earliestTime = schedulingDeadline
		}
	}
	if earliestTime.After(now) {
		return time.Time{}, sched.Next(now), nil
	}

	starts := 0
	lastMissed := time.Time{}
	for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
		lastMissed = t
		starts++
		if starts > 100 {
			// We can't get the most recent times so just return an empty slice
			// This may occur due to low startingDeadlineSeconds, clock skew or
			// when user has reduced the schedule which in turn
			// leads to the new interval being less than 100 times of the old interval.
			return time.Time{}, time.Time{},
				fmt.Errorf("too many missed start times (> 100). Set or decrease" +
					".spec.startingDeadlineSeconds, check clock skew or" +
					" delete and recreate encryptionkeyrotationjob")
		}
	}
	return lastMissed, sched.Next(now), nil
}

// constructEncryptionKeyRotationJob forms an EncryptionKeyRotationJob for a given
// EncryptionKeyRotationCronJob
func (r *EncryptionKeyRotationCronJobReconciler) constructEncryptionKeyRotationJob(
	krcJob *csiaddonsv1alpha1.EncryptionKeyRotationCronJob,
	schedTime time.Time) (*csiaddonsv1alpha1.EncryptionKeyRotationJob, error) {
	name := fmt.Sprintf("%s-%d", krcJob.Name, schedTime.Unix())

	job := &csiaddonsv1alpha1.EncryptionKeyRotationJob{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   krcJob.Namespace,
		},
		Spec: *krcJob.Spec.JobSpec.Spec.DeepCopy(),
	}
	for k, v := range krcJob.Spec.JobSpec.Annotations {
		job.Annotations[k] = v
	}
	job.Annotations[scheduledTimeAnnotation] = schedTime.Format(time.RFC3339)
	for k, v := range krcJob.Spec.JobSpec.Labels {
		job.Labels[k] = v
	}
	if err := ctrl.SetControllerReference(krcJob, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EncryptionKeyRotationCronJobReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &csiaddonsv1alpha1.EncryptionKeyRotationJob{}, jobOwnerKey, func(rawObj client.Object) []string {
		job, ok := rawObj.(*csiaddonsv1alpha1.EncryptionKeyRotationJob)
		if !ok {
			return nil
		}
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != "EncryptionKeyRotationCronJob" {
			return nil
		}

		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}).
		Owns(&csiaddonsv1alpha1.EncryptionKeyRotationJob{}).
		WithOptions(ctrlOptions).
		Complete(r)
}
