package db

import (
	"context"

	"github.com/babylonchain/cli-tools/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Database struct {
	DbName string
	Client *mongo.Client
}

func New(ctx context.Context, dbName string, dbURI string) (*Database, error) {
	clientOps := options.Client().ApplyURI(dbURI)
	client, err := mongo.Connect(ctx, clientOps)
	if err != nil {
		return nil, err
	}

	return &Database{
		DbName: dbName,
		Client: client,
	}, nil
}

func (db *Database) Ping(ctx context.Context) error {
	err := db.Client.Ping(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) SaveUnbondingDocument(
	ctx context.Context,
	unbondingTxHashHex string,
	unbondingTxHex string,
	unbondingTxSigHex string,
	stakerPkHex string,
	finalityPkHex string,
	stakingTime uint64,
	stakingAmount uint64,
) error {
	client := db.Client.Database(db.DbName).Collection(model.UnbondingCollection)
	document := model.UnbondingDocument{
		ID:                 primitive.NewObjectID(),
		UnbondingTxHashHex: unbondingTxHashHex,
		UnbondingTxHex:     unbondingTxHex,
		UnbondingTxSigHex:  unbondingTxSigHex,
		StakerPkHex:        stakerPkHex,
		FinalityPkHex:      finalityPkHex,
		StakingTimelock:    stakingTime,
		StakingAmount:      stakingAmount,
		State:              model.Inserted,
	}
	_, err := client.InsertOne(ctx, document)

	return err

}

func (db *Database) FindNewUnbondingDocuments(ctx context.Context) ([]model.UnbondingDocument, error) {
	client := db.Client.Database(db.DbName).Collection(model.UnbondingCollection)

	filter := bson.M{"state": model.Inserted}
	options := options.Find().SetSort(bson.M{"_id": 1}) // Sorting in ascending order

	cursor, err := client.Find(ctx, filter, options)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var delegations []model.UnbondingDocument
	if err = cursor.All(ctx, &delegations); err != nil {
		return nil, err
	}

	return delegations, nil
}

func (db *Database) updateUnbondingDocumentState(
	ctx context.Context,
	unbondingTxHashHex string,
	newState model.UnbondingState) error {
	client := db.Client.Database(db.DbName).Collection(model.UnbondingCollection)
	filter := bson.M{"unbonding_tx_hash_hex": unbondingTxHashHex}
	update := bson.M{"$set": bson.M{"state": newState}}
	_, err := client.UpdateOne(ctx, filter, update)
	return err
}

func (db *Database) SetUnbondingDocumentSend(
	ctx context.Context,
	unbondingTxHashHex string) error {
	return db.updateUnbondingDocumentState(ctx, unbondingTxHashHex, model.Send)
}

func (db *Database) SetUnbondingDocumentFailed(
	ctx context.Context,
	unbondingTxHashHex string) error {
	return db.updateUnbondingDocumentState(ctx, unbondingTxHashHex, model.Failed)
}
