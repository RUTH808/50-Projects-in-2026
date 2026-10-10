package dev.campusflow;

import java.sql.Timestamp;
import java.time.Instant;
import java.util.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class CampusService {

  private final JdbcTemplate db;

  public CampusService(JdbcTemplate db) {
    this.db = db;
  }

  public List<Map<String, Object>> rows(String sql, Object... args) {
    var rows = db.queryForList(sql, args);
    rows.forEach(row ->
      row.replaceAll((key, value) ->
        value instanceof Timestamp t ? t.toInstant().toString() : value
      )
    );
    return rows;
  }

  public Map<String, Object> actor(String email) {
    return rows("SELECT id,name,email,role FROM app_user WHERE email=?", email)
      .stream()
      .findFirst()
      .orElseThrow(() -> new DomainException(401, "Unknown account"));
  }

  private Map<String, Object> equipment(long id) {
    return rows("SELECT * FROM equipment WHERE id=? FOR UPDATE", id)
      .stream()
      .findFirst()
      .orElseThrow(() -> new DomainException(404, "Equipment not found"));
  }

  private Map<String, Object> booking(long id) {
    return rows("SELECT * FROM booking WHERE id=?", id)
      .stream()
      .findFirst()
      .orElseThrow(() -> new DomainException(404, "Booking not found"));
  }

  private long number(Map<String, Object> row, String key) {
    return ((Number) row.get(key)).longValue();
  }

  private boolean overlaps(long equipmentId, Instant start, Instant end) {
    return db.queryForObject(
      "SELECT EXISTS(SELECT 1 FROM booking WHERE equipment_id=? AND status IN ('PENDING','CONFIRMED','CHECKED_OUT') AND starts_at < ? AND ends_at > ?)",
      Boolean.class,
      equipmentId,
      Timestamp.from(end),
      Timestamp.from(start)
    );
  }

  private void audit(long actor, String action, String entity, long id) {
    db.update(
      "INSERT INTO audit_event(actor_id,action,entity_type,entity_id) VALUES (?,?,?,?)",
      actor,
      action,
      entity,
      id
    );
  }

  @Transactional
  public Map<String, Object> reserve(
    long user,
    long equipmentId,
    Instant start,
    Instant end,
    String purpose
  ) {
    if (
      !start.isAfter(Instant.now()) ||
      !end.isAfter(start) ||
      end.isAfter(start.plusSeconds(7 * 86400))
    ) throw new DomainException(
      400,
      "Choose a future interval of at most seven days"
    );
    var item = equipment(equipmentId);
    if (
      db.queryForObject(
        "SELECT EXISTS(SELECT 1 FROM booking WHERE user_id=? AND equipment_id=? AND starts_at=? AND ends_at=? AND status IN ('PENDING','CONFIRMED','WAITLISTED','CHECKED_OUT'))",
        Boolean.class,
        user,
        equipmentId,
        Timestamp.from(start),
        Timestamp.from(end)
      )
    ) throw new DomainException(409, "You already requested this interval");
    String status = Boolean.TRUE.equals(item.get("maintenance")) ||
      overlaps(equipmentId, start, end)
      ? "WAITLISTED"
      : "PENDING";
    long id = db.queryForObject(
      "INSERT INTO booking(equipment_id,user_id,starts_at,ends_at,purpose,status) VALUES (?,?,?,?,?,?) RETURNING id",
      Long.class,
      equipmentId,
      user,
      Timestamp.from(start),
      Timestamp.from(end),
      purpose,
      status
    );
    audit(user, "BOOKING_" + status, "booking", id);
    return booking(id);
  }

  // Caller holds the equipment row lock. Oldest eligible request wins; incompatible requests remain queued.
  private void promote(long equipmentId, long actor) {
    if (Boolean.TRUE.equals(equipment(equipmentId).get("maintenance"))) return;
    for (var row : rows(
      "SELECT * FROM booking WHERE equipment_id=? AND status='WAITLISTED' ORDER BY created_at,id",
      equipmentId
    )) {
      long id = number(row, "id");
      Instant start = Instant.parse((String) row.get("starts_at")),
        end = Instant.parse((String) row.get("ends_at"));
      if (!start.isAfter(Instant.now())) {
        db.update("UPDATE booking SET status='CANCELLED' WHERE id=?", id);
        audit(actor, "WAITLIST_EXPIRED", "booking", id);
      } else if (!overlaps(equipmentId, start, end)) {
        db.update("UPDATE booking SET status='PENDING' WHERE id=?", id);
        audit(actor, "WAITLIST_PROMOTED", "booking", id);
      }
    }
  }

  @Transactional
  public Map<String, Object> transition(
    long id,
    String action,
    Map<String, Object> actor
  ) {
    var initial = booking(id);
    long equipmentId = number(initial, "equipment_id");
    var item = equipment(equipmentId);
    var row = booking(id); // Re-read after locking to avoid stale decisions.
    long actorId = number(actor, "id");
    boolean staff = !actor.get("role").equals("STUDENT");
    if (!staff && number(row, "user_id") != actorId) throw new DomainException(
      403,
      "This booking belongs to another user"
    );
    String status = (String) row.get("status"),
      next;
    switch (action) {
      case "cancel" -> {
        if (
          !Set.of("PENDING", "CONFIRMED", "WAITLISTED").contains(status)
        ) throw new DomainException(409, "Booking cannot be cancelled now");
        next = "CANCELLED";
      }
      case "approve" -> {
        if (!staff) throw new DomainException(403, "Staff approval required");
        if (
          !status.equals("PENDING") ||
          Boolean.TRUE.equals(item.get("maintenance"))
        ) throw new DomainException(409, "Booking is not approvable");
        if (
          !Instant.parse((String) row.get("ends_at")).isAfter(Instant.now())
        ) throw new DomainException(409, "Booking has expired");
        next = "CONFIRMED";
      }
      case "reject" -> {
        if (!staff) throw new DomainException(403, "Staff approval required");
        if (!status.equals("PENDING")) throw new DomainException(
          409,
          "Only pending requests can be rejected"
        );
        next = "REJECTED";
      }
      case "checkout" -> {
        if (!staff) throw new DomainException(403, "Staff checkout required");
        Instant now = Instant.now();
        if (
          !status.equals("CONFIRMED") ||
          Boolean.TRUE.equals(item.get("maintenance")) ||
          now.isBefore(Instant.parse((String) row.get("starts_at"))) ||
          !now.isBefore(Instant.parse((String) row.get("ends_at")))
        ) throw new DomainException(
          409,
          "Checkout requires a confirmed booking within its time window and serviceable equipment"
        );
        if (
          db.queryForObject(
            "SELECT EXISTS(SELECT 1 FROM booking WHERE equipment_id=? AND status='CHECKED_OUT')",
            Boolean.class,
            equipmentId
          )
        ) throw new DomainException(
          409,
          "Equipment has not been returned by its previous borrower"
        );
        next = "CHECKED_OUT";
      }
      case "return" -> {
        if (!staff) throw new DomainException(403, "Staff return required");
        if (!status.equals("CHECKED_OUT")) throw new DomainException(
          409,
          "Equipment is not checked out"
        );
        next = "RETURNED";
      }
      default -> throw new DomainException(400, "Unknown booking action");
    }
    db.update("UPDATE booking SET status=? WHERE id=?", next, id);
    audit(actorId, "BOOKING_" + next, "booking", id);
    if (Set.of("CANCELLED", "REJECTED", "RETURNED").contains(next)) promote(
      equipmentId,
      actorId
    );
    return booking(id);
  }

  @Transactional
  public Map<String, Object> report(
    long actor,
    long equipmentId,
    String description
  ) {
    equipment(equipmentId);
    long id = db.queryForObject(
      "INSERT INTO maintenance_ticket(equipment_id,reported_by,description) VALUES (?,?,?) RETURNING id",
      Long.class,
      equipmentId,
      actor,
      description
    );
    db.update("UPDATE equipment SET maintenance=true WHERE id=?", equipmentId);
    audit(actor, "MAINTENANCE_REPORTED", "maintenance_ticket", id);
    return rows("SELECT * FROM maintenance_ticket WHERE id=?", id).get(0);
  }

  @Transactional
  public void resolve(long actor, long ticketId) {
    var initial = rows("SELECT * FROM maintenance_ticket WHERE id=?", ticketId)
      .stream()
      .findFirst()
      .orElseThrow(() -> new DomainException(404, "Ticket not found"));
    long equipmentId = number(initial, "equipment_id");
    equipment(equipmentId);
    if (
      db.update(
        "UPDATE maintenance_ticket SET status='RESOLVED',resolved_at=now() WHERE id=? AND status='OPEN'",
        ticketId
      ) ==
      0
    ) throw new DomainException(409, "Ticket already resolved");
    db.update(
      "UPDATE equipment SET maintenance=EXISTS(SELECT 1 FROM maintenance_ticket WHERE equipment_id=? AND status='OPEN') WHERE id=?",
      equipmentId,
      equipmentId
    );
    audit(actor, "MAINTENANCE_RESOLVED", "maintenance_ticket", ticketId);
    promote(equipmentId, actor);
  }

  @Transactional
  public Map<String, Object> addEquipment(
    long actor,
    String name,
    String category,
    String location,
    String description
  ) {
    long id = db.queryForObject(
      "INSERT INTO equipment(name,category,location,description) VALUES (?,?,?,?) RETURNING id",
      Long.class,
      name,
      category,
      location,
      description
    );
    audit(actor, "EQUIPMENT_CREATED", "equipment", id);
    return rows("SELECT * FROM equipment WHERE id=?", id).get(0);
  }
}
