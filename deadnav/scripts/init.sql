-- Database initialization script for Task Scheduler

CREATE DATABASE IF NOT EXISTS task_scheduler CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE task_scheduler;

-- Tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status ENUM('pending', 'in_progress', 'completed', 'cancelled') DEFAULT 'pending',
    priority INT DEFAULT 1 CHECK (priority >= 1 AND priority <= 5),
    start_date DATETIME NOT NULL,
    end_date DATETIME NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_status (status),
    INDEX idx_priority (priority),
    INDEX idx_dates (start_date, end_date)
);

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_username (username),
    INDEX idx_email (email)
);

-- Schedules table
CREATE TABLE IF NOT EXISTS schedules (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_task_id (task_id),
    INDEX idx_user_id (user_id),
    INDEX idx_time_range (start_time, end_time)
);

-- Statistics view (optional, for quick stats)
CREATE OR REPLACE VIEW task_statistics AS
SELECT 
    COUNT(*) as total_tasks,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_tasks,
    SUM(CASE WHEN status IN ('pending', 'in_progress') THEN 1 ELSE 0 END) as pending_tasks,
    AVG(CASE WHEN status = 'completed' THEN TIMESTAMPDIFF(HOUR, start_date, end_date) ELSE NULL END) as avg_duration_hours
FROM tasks;

-- Insert sample data (optional)
INSERT INTO tasks (title, description, status, priority, start_date, end_date) VALUES
('Project Planning', 'Initial project planning and requirements gathering', 'completed', 1, NOW() - INTERVAL 10 DAY, NOW() - INTERVAL 7 DAY),
('Database Design', 'Design database schema and relationships', 'completed', 2, NOW() - INTERVAL 7 DAY, NOW() - INTERVAL 5 DAY),
('API Development', 'Develop REST API endpoints', 'in_progress', 1, NOW() - INTERVAL 5 DAY, NOW() + INTERVAL 5 DAY),
('Frontend Development', 'Build user interface components', 'pending', 2, NOW() + INTERVAL 2 DAY, NOW() + INTERVAL 15 DAY),
('Testing', 'Perform unit and integration testing', 'pending', 3, NOW() + INTERVAL 10 DAY, NOW() + INTERVAL 20 DAY);
