package dev.campusflow;

import jakarta.validation.Valid;
import jakarta.validation.constraints.*;
import java.security.Principal;
import java.time.Instant;
import java.util.*;
import org.springframework.security.web.csrf.CsrfToken;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api")
public class ApiController {

  private final CampusService service;

  public ApiController(CampusService service) {
    this.service = service;
  }

  private Map<String, Object> actor(Principal p) {
    return service.actor(p.getName());
  }

  private long actorId(Principal p) {
    return ((Number) actor(p).get("id")).longValue();
  }

  public record Reservation(
    @Positive long equipmentId,
    @NotNull Instant startsAt,
    @NotNull Instant endsAt,
    @NotBlank @Size(max = 500) String purpose
  ) {}

  public record Report(
    @Positive long equipmentId,
    @NotBlank @Size(max = 2000) String description
  ) {}

  public record Equipment(
    @NotBlank @Size(max = 120) String name,
    @NotBlank @Size(max = 80) String category,
    @NotBlank @Size(max = 200) String location,
    @NotNull @Size(max = 1000) String description
  ) {}

  @GetMapping("/csrf")
  Map<String, String> csrf(CsrfToken token) {
    return Map.of("token", token.getToken());
  }

  @GetMapping("/me")
  Map<String, Object> me(Principal p) {
    return actor(p);
  }

  @GetMapping("/equipment")
  List<Map<String, Object>> equipment() {
    return service.rows(
      "SELECT * FROM equipment ORDER BY category,name LIMIT 1000"
    );
  }

  @GetMapping("/bookings")
  List<Map<String, Object>> bookings(Principal p) {
    var a = actor(p);
    String sql =
      "SELECT b.*,e.name AS equipment_name,u.name AS user_name FROM booking b JOIN equipment e ON e.id=b.equipment_id JOIN app_user u ON u.id=b.user_id";
    return a.get("role").equals("STUDENT")
      ? service.rows(
        sql + " WHERE b.user_id=? ORDER BY b.created_at DESC LIMIT 500",
        a.get("id")
      )
      : service.rows(sql + " ORDER BY b.created_at DESC LIMIT 500");
  }

  @PostMapping("/bookings")
  Map<String, Object> reserve(
    Principal p,
    @Valid @RequestBody Reservation request
  ) {
    return service.reserve(
      actorId(p),
      request.equipmentId(),
      request.startsAt(),
      request.endsAt(),
      request.purpose()
    );
  }

  @PostMapping("/bookings/{id}/{action}")
  Map<String, Object> transition(
    Principal p,
    @PathVariable long id,
    @PathVariable String action
  ) {
    return service.transition(id, action, actor(p));
  }

  @PostMapping("/maintenance")
  Map<String, Object> report(Principal p, @Valid @RequestBody Report request) {
    return service.report(
      actorId(p),
      request.equipmentId(),
      request.description()
    );
  }

  @GetMapping("/staff/maintenance")
  List<Map<String, Object>> maintenance() {
    return service.rows(
      "SELECT t.*,e.name AS equipment_name FROM maintenance_ticket t JOIN equipment e ON e.id=t.equipment_id ORDER BY t.created_at DESC LIMIT 500"
    );
  }

  @PostMapping("/staff/maintenance/{id}/resolve")
  Map<String, String> resolve(Principal p, @PathVariable long id) {
    service.resolve(actorId(p), id);
    return Map.of("status", "RESOLVED");
  }

  @PostMapping("/admin/equipment")
  Map<String, Object> add(Principal p, @Valid @RequestBody Equipment request) {
    return service.addEquipment(
      actorId(p),
      request.name(),
      request.category(),
      request.location(),
      request.description()
    );
  }

  @GetMapping("/staff/audit")
  List<Map<String, Object>> audit() {
    return service.rows(
      "SELECT a.*,u.name AS actor_name FROM audit_event a JOIN app_user u ON a.actor_id=u.id ORDER BY a.id DESC LIMIT 200"
    );
  }

  @GetMapping("/staff/reports")
  List<Map<String, Object>> reports() {
    return service.rows(
      "SELECT e.id,e.name,e.maintenance,count(b.id) FILTER (WHERE b.status='WAITLISTED') AS waiting,count(b.id) FILTER (WHERE b.status IN ('CONFIRMED','CHECKED_OUT','RETURNED')) AS approved_bookings,COALESCE(sum(EXTRACT(EPOCH FROM (b.ends_at-b.starts_at))/3600) FILTER (WHERE b.status IN ('CONFIRMED','CHECKED_OUT','RETURNED')),0) AS reserved_hours FROM equipment e LEFT JOIN booking b ON b.equipment_id=e.id AND b.created_at>now()-interval '30 days' GROUP BY e.id ORDER BY e.name"
    );
  }
}
