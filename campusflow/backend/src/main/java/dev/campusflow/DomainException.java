package dev.campusflow;

public class DomainException extends RuntimeException {

  public final int status;

  public DomainException(int status, String message) {
    super(message);
    this.status = status;
  }
}
