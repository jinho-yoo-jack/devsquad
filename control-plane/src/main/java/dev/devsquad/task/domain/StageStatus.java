package dev.devsquad.task.domain;

/** 11-시스템-아키텍처 §5.2 Stage 상태. */
public enum StageStatus {
    PENDING, PLANNING, PLAN_REVIEW, EXECUTING, DELIVERABLE_REVIEW, APPROVED, BLOCKED
}
