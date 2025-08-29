-- Sample data for MariaDB Extractor testing
-- This file is automatically executed when the MariaDB container starts

-- Create a sample e-commerce database
CREATE DATABASE IF NOT EXISTS ecommerce;
USE ecommerce;

-- Users table
CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(50),
    last_name VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Products table
CREATE TABLE products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    category VARCHAR(100),
    stock_quantity INT DEFAULT 0,
    sku VARCHAR(50) UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Orders table
CREATE TABLE orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10,2) NOT NULL,
    status ENUM('pending', 'processing', 'shipped', 'delivered', 'cancelled') DEFAULT 'pending',
    shipping_address TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Order items table
CREATE TABLE order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity INT NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    total_price DECIMAL(10,2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insert sample data
INSERT INTO users (username, email, password_hash, first_name, last_name) VALUES
('john_doe', 'john@example.com', 'hashed_password_1', 'John', 'Doe'),
('jane_smith', 'jane@example.com', 'hashed_password_2', 'Jane', 'Smith'),
('bob_wilson', 'bob@example.com', 'hashed_password_3', 'Bob', 'Wilson');

INSERT INTO products (name, description, price, category, stock_quantity, sku) VALUES
('Wireless Headphones', 'High-quality wireless headphones with noise cancellation', 199.99, 'Electronics', 50, 'WH-001'),
('Smart Watch', 'Fitness tracking smartwatch with heart rate monitor', 299.99, 'Electronics', 30, 'SW-002'),
('Coffee Maker', 'Programmable coffee maker with thermal carafe', 89.99, 'Appliances', 25, 'CM-003'),
('Yoga Mat', 'Non-slip yoga mat for home workouts', 29.99, 'Sports', 100, 'YM-004');

INSERT INTO orders (user_id, total_amount, status, shipping_address) VALUES
(1, 229.98, 'processing', '123 Main St, Anytown, USA'),
(2, 299.99, 'shipped', '456 Oak Ave, Somewhere, USA');

INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES
(1, 1, 1, 199.99),
(1, 4, 1, 29.99),
(2, 2, 1, 299.99);

-- Create a sample blog database
CREATE DATABASE IF NOT EXISTS blog;
USE blog;

-- Posts table
CREATE TABLE posts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    content TEXT,
    excerpt VARCHAR(500),
    author_id INT,
    published BOOLEAN DEFAULT FALSE,
    published_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_published (published),
    INDEX idx_slug (slug)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Categories table
CREATE TABLE categories (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Post categories junction table
CREATE TABLE post_categories (
    post_id INT NOT NULL,
    category_id INT NOT NULL,
    PRIMARY KEY (post_id, category_id),
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insert sample blog data
INSERT INTO categories (name, slug, description) VALUES
('Technology', 'technology', 'Posts about technology and programming'),
('Lifestyle', 'lifestyle', 'Lifestyle and personal development posts'),
('Travel', 'travel', 'Travel experiences and tips');

INSERT INTO posts (title, slug, content, excerpt, published, published_at) VALUES
('Getting Started with Docker', 'getting-started-with-docker',
 'Docker is a platform for developing, shipping, and running applications in containers...',
 'Learn the basics of Docker containerization', TRUE, NOW()),
('Healthy Living Tips', 'healthy-living-tips',
 'Maintaining a healthy lifestyle involves several key aspects...',
 'Essential tips for maintaining good health', TRUE, NOW() - INTERVAL 2 DAY),
('My Trip to Japan', 'my-trip-to-japan',
 'Japan is a fascinating country with rich culture and traditions...',
 'An amazing journey through Japan', FALSE, NULL);

INSERT INTO post_categories (post_id, category_id) VALUES
(1, 1), -- Docker post -> Technology
(2, 2), -- Health post -> Lifestyle
(3, 3); -- Japan post -> Travel

-- Create an empty database for testing
CREATE DATABASE IF NOT EXISTS empty_db;

-- Create a system database for testing exclusions
CREATE DATABASE IF NOT EXISTS test_system_db;