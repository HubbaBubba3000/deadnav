-- Deadnav database initialisation
-- Runs as root during container init.

-- Create database first
CREATE DATABASE IF NOT EXISTS deadnav;
USE deadnav;

-- Create deadnav_user (requires database to exist for grants)
CREATE USER IF NOT EXISTS 'deadnav_user'@'%' IDENTIFIED BY 'deadnav_password';
GRANT ALL PRIVILEGES ON deadnav.* TO 'deadnav_user'@'%';
FLUSH PRIVILEGES;

CREATE TABLE IF NOT EXISTS users (
    id            BIGINT AUTO_INCREMENT PRIMARY KEY,
    username      VARCHAR(255)  NOT NULL,
    email         VARCHAR(255)  NOT NULL DEFAULT '',
    password_hash VARCHAR(255)  NOT NULL DEFAULT '',
    telegram_id   BIGINT        NULL     UNIQUE,
    auth_provider VARCHAR(50)   NOT NULL DEFAULT 'local' COMMENT 'local | telegram',
    created_at    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uk_username (username),
    UNIQUE KEY uk_email    (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tasks (
    id               BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id          BIGINT       NOT NULL,
    title            VARCHAR(255) NOT NULL,
    description      TEXT         NULL,
    status           VARCHAR(50)  NOT NULL DEFAULT 'pending'
                     COMMENT 'pending | in_progress | completed | cancelled',
    priority         INT          NOT NULL DEFAULT 3  COMMENT '1(low) – 5(high)',
    duration_minutes INT          NOT NULL DEFAULT 0  COMMENT '0 = auto-calculated',
    start_date       DATETIME     NOT NULL COMMENT 'earliest start / window open',
    end_date         DATETIME     NOT NULL COMMENT 'deadline',
    complexity       INT          NOT NULL DEFAULT 3  COMMENT '1–5',
    urgency          INT          NOT NULL DEFAULT 3  COMMENT '1–5',
    importance       INT          NOT NULL DEFAULT 3  COMMENT '1–5',
    estimated_time   INT          NOT NULL DEFAULT 0  COMMENT 'minutes',
    created_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_user_status (user_id, status),
    INDEX idx_user_dates  (user_id, start_date, end_date),

    CONSTRAINT fk_tasks_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS schedules (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id     BIGINT   NOT NULL UNIQUE,
    user_id     BIGINT   NOT NULL,
    start_time  DATETIME NOT NULL,
    end_time    DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_sched_user_timerange (user_id, start_time, end_time),

    CONSTRAINT fk_schedules_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    CONSTRAINT fk_schedules_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id          BIGINT       PRIMARY KEY,
    work_start_hour  INT          NOT NULL DEFAULT 9,
    work_end_hour    INT          NOT NULL DEFAULT 18,
    work_days        VARCHAR(100) NOT NULL DEFAULT 'Mon,Tue,Wed,Thu,Fri',
    min_slot_minutes INT          NOT NULL DEFAULT 30,
    timezone         VARCHAR(50)  NOT NULL DEFAULT 'UTC',

    CONSTRAINT fk_prefs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
