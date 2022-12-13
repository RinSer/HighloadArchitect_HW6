CREATE TABLE IF NOT EXISTS users (
    id    BIGINT AUTO_INCREMENT PRIMARY KEY,
    login VARCHAR(25)
);
--
CREATE TABLE IF NOT EXISTS followers (
    userId      BIGINT,
    followerId  BIGINT,
    PRIMARY KEY (userId, followerId),
    FOREIGN KEY (userId) REFERENCES users(id),
    FOREIGN KEY (followerId) REFERENCES users(id)
);
--
CREATE TABLE IF NOT EXISTS publications (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    author      BIGINT,
    txt         VARCHAR(512),
    createdAt   TIMESTAMP,
    FOREIGN KEY (author) REFERENCES users(id)
);