package dev.devsquad.common;

/** 404 <RESOURCE>_NOT_FOUND. */
public class NotFoundException extends RuntimeException {
    private final String code;

    public NotFoundException(String resource, Object id) {
        super(resource + " not found: " + id);
        this.code = resource.toUpperCase() + "_NOT_FOUND";
    }

    public String code() { return code; }
}
