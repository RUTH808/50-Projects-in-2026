package dev.campusflow;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.CommandLineRunner;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

@Component
public class DemoSeed implements CommandLineRunner {

  private final JdbcTemplate db;
  private final PasswordEncoder encoder;

  @Value("${campusflow.demo}")
  boolean demo;

  @Value("${campusflow.demo-password}")
  String password;

  public DemoSeed(JdbcTemplate db, PasswordEncoder encoder) {
    this.db = db;
    this.encoder = encoder;
  }

  @Override
  @Transactional
  public void run(String... args) {
    if (!demo) return;
    for (String role : new String[] { "STUDENT", "STAFF", "ADMIN" }) {
      db.update(
        "INSERT INTO app_user(name,email,password_hash,role) VALUES (?,?,?,?) ON CONFLICT(email) DO NOTHING",
        role.charAt(0) + role.substring(1).toLowerCase(),
        role.toLowerCase() + "@campusflow.local",
        encoder.encode(password),
        role
      );
    }
    if (db.queryForObject("SELECT count(*) FROM equipment", Long.class) == 0) {
      db.update(
        "INSERT INTO equipment(name,category,location,description) VALUES (?,?,?,?)",
        "Sony Alpha Camera",
        "Media",
        "Media Lab · Block A",
        "Camera kit with two lenses, tripod and batteries."
      );
      db.update(
        "INSERT INTO equipment(name,category,location,description) VALUES (?,?,?,?)",
        "Arduino Development Kit",
        "Electronics",
        "Embedded Lab · Block B",
        "Board, sensors and breadboard for hardware prototypes."
      );
      db.update(
        "INSERT INTO equipment(name,category,location,description) VALUES (?,?,?,?)",
        "Oscilloscope",
        "Electronics",
        "Circuits Lab · Block B",
        "Four-channel digital oscilloscope for lab experiments."
      );
      db.update(
        "INSERT INTO equipment(name,category,location,description) VALUES (?,?,?,?)",
        "Workshop Room",
        "Spaces",
        "Innovation Centre",
        "Shared workshop room with projector; capacity 20."
      );
    }
  }
}
