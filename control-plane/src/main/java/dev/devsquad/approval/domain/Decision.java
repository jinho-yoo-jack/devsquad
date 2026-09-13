package dev.devsquad.approval.domain;

import java.util.Objects;

/** 15-API §1 POST /approvals/{id}/decide 요청. reject 는 feedback 필수, edit 은 edited_content 필수. */
public record Decision(Kind kind, String feedback, String editedContent) {

    public enum Kind { APPROVE, REJECT, EDIT }

    public Decision {
        Objects.requireNonNull(kind, "kind");
        if (kind == Kind.REJECT && (feedback == null || feedback.isBlank())) {
            throw new FeedbackRequiredException();
        }
        if (kind == Kind.EDIT && (editedContent == null || editedContent.isBlank())) {
            throw new IllegalArgumentException("edit 에는 edited_content 가 필요합니다");
        }
    }

    public static Decision approve() { return new Decision(Kind.APPROVE, null, null); }
    public static Decision reject(String feedback) { return new Decision(Kind.REJECT, feedback, null); }
    public static Decision edit(String editedContent) { return new Decision(Kind.EDIT, null, editedContent); }

    public ApprovalStatus resultingStatus() {
        return switch (kind) {
            case APPROVE -> ApprovalStatus.APPROVED;
            case REJECT -> ApprovalStatus.REJECTED;
            case EDIT -> ApprovalStatus.EDITED;
        };
    }

    /** Agent Runtime resume 페이로드의 decision 값 (15-API §4). */
    public String wireValue() {
        return kind.name().toLowerCase();
    }
}
